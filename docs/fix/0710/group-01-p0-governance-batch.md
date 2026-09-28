# G01 · 启动装配与运行依赖编排

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G01 |
| 问题数量 | 10 |
| 整体优先级 | P0 |
| 涉及模块 | `internal/boot`、`internal/agent` |
| 涉及文件 | `internal/boot/boot.go`<br>`internal/agent/migrate.go`<br>`internal/boot/token_profile.go` |
| 主要治理目标 | 围绕 `internal/agent/migrate.go` 等治理对象降低复杂度/体量，统一处理 2 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | boot / agent 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614`、`origin/expert-202607081622`；单文件最多集中 12 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 1 组；先于 G05 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 已完成 / 已验证 |

### 为什么排在当前顺序

本组最高治理得分为 **83/100**，对象 `Build` 的分项为严重度 30.0、集中度 15.9、影响面 29.2、可执行性 8.0；最高超限 17.80x，文件内 5 条问题，CodeGraph 记录 11 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数、嵌套深度，必须合并治理。

### 治理边界与组间关系

本组涉及 3 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G05，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-079` | `complexity\|internal/boot/boot.go\|Build` | 圈复杂度 | `internal/boot/boot.go` | `Build` | 152 | 178/10 | +168（17.80x） | P0 | 11 可信 + 0 待核 caller / 131 可信 + 46 待核 callee / 148 unresolved | 待修改 |
| `FN-011` | `func_lines\|internal/boot/boot.go\|Build` | 函数有效行数 | `internal/boot/boot.go` | `Build` | 152 | 924/80 | +844（11.55x） | P0 | 11 可信 + 0 待核 caller / 131 可信 + 46 待核 callee / 148 unresolved | 待修改 |
| `ND-005` | `nesting_depth\|internal/boot/boot.go\|Build` | 嵌套深度 | `internal/boot/boot.go` | `Build` | 152 | 7/4 | +3（1.75x） | P0 | 11 可信 + 0 待核 caller / 131 可信 + 46 待核 callee / 148 unresolved | 待修改 |
| `CX-032` | `complexity\|internal/agent/migrate.go\|migrateLegacySessions` | 圈复杂度 | `internal/agent/migrate.go` | `migrateLegacySessions` | 82 | 54/10 | +44（5.40x） | P1 | 2 可信 + 0 待核 caller / 11 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `FN-006` | `func_lines\|internal/agent/migrate.go\|migrateLegacySessions` | 函数有效行数 | `internal/agent/migrate.go` | `migrateLegacySessions` | 82 | 146/80 | +66（1.82x） | P1 | 2 可信 + 0 待核 caller / 11 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `ND-003` | `nesting_depth\|internal/agent/migrate.go\|migrateLegacySessions` | 嵌套深度 | `internal/agent/migrate.go` | `migrateLegacySessions` | 82 | 5/4 | +1（1.25x） | P1 | 2 可信 + 0 待核 caller / 11 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `FL-004` | `file_lines\|internal/boot/boot.go\|` | 文件有效行数 | `internal/boot/boot.go` | `internal/boot/boot.go` | 1 | 1481/800 | +681（1.85x） | P2 | 7 可信 + 0 待核 caller / 24 可信 + 16 待核 callee / 253 unresolved | 待修改 |
| `CX-037` | `complexity\|internal/agent/migrate.go\|rehomeStrandedSessions` | 圈复杂度 | `internal/agent/migrate.go` | `rehomeStrandedSessions` | 685 | 20/10 | +10（2.00x） | P2 | 1 可信 + 0 待核 caller / 7 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-081` | `complexity\|internal/boot/token_profile.go\|*toolSourceConnector.Execute` | 圈复杂度 | `internal/boot/token_profile.go` | `*toolSourceConnector.Execute` | 108 | 15/10 | +5（1.50x） | P3 | 0 可信 + 0 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-080` | `complexity\|internal/boot/boot.go\|subagentEffectiveIdentity` | 圈复杂度 | `internal/boot/boot.go` | `subagentEffectiveIdentity` | 1454 | 14/10 | +4（1.40x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 1 待核 callee / 9 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-079 · 圈复杂度 · `Build`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-079 |
| Baseline key | `complexity\|internal/boot/boot.go\|Build` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P0（原始 P0，治理对象得分 83） |
| 文件 | `internal/boot/boot.go` |
| 代码行 | 152 |
| 函数/方法/包 | `Build` |
| Baseline / 当前复测 / 阈值 | 178 / 178 / 10 |
| 超出 | +168，17.80x |
| 同一治理对象关联问题 | `FN-011`、`ND-005` |

#### 明确的问题描述

`Build` 的圈复杂度当前为 **178**，超过限制 **10**（+168，17.80x）。定义范围为 152-1227 行；可见控制流热点：L154 `if stderr == nil {`；L330 `if err != nil {`；L471 `for _, t := range tools {`；L677 `if !tokenEconomy {`；L937 `for _, t := range skill.BuiltinSubagentTools(skillStore, skillRunner, skillProfile) {`；L1223 `if classifier != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（resolveSubagentProvider、ResolveSelection、TaskTool::WithTranscriptIdentityResolver、ResolveAgentMode）、状态与控制（RunSubAgentWithSession、EphemeralSubagentRun、FormatSubagentRunResult、BuildRunner）、持久化/I-O（ArchiveDir、CodeGraphReadTools、ReadOnlyIndexBlock、NewReadOnlyTaskTool）、渲染/输出（FormatSubagentRunResult、missingAPIKeyNotice、MCPStartupNotice、Render）、网络/协议（newProviderWithProxyForOwner、resolveSubagentProvider、NormalizeLegacyMimoCustomProvidersForRefs、NewBillableProviderAutoPlanClassifier）、权限/安全（PluginSpecsForRootWithPlanModeAllowedTools、rememberPermissionRuleForActor、applyPlanModeAllowedMCPToolTrust、planModeTrustedMCPServers）、任务/并发（IndexBlock、ReadOnlyIndexBlock、NewParallelTasksTool、NewTaskTool）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function Build（internal/boot/boot.go:152-1227）
- **可信生产调用方（11）：** `App::clearActiveSessionRuntime`（`desktop/app.go:1400`，调用点 L1429，confidence=0.90，import）；`App::SetModelForTab`（`desktop/app.go:6025`，调用点 L6097，confidence=0.90，import）；`App::SetEffortForTab`（`desktop/app.go:6159`，调用点 L6200，confidence=0.90，import）；`App::SetTokenModeForTab`（`desktop/app.go:6240`，调用点 L6276，confidence=0.90，import）；`App::rebuild`（`desktop/settings_app.go:901`，调用点 L932，confidence=0.90，import）；`App::buildTabControllerWithContext`（`desktop/tabs.go:2144`，调用点 L2275，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（131）：** `newProviderWithProxyForOwner`（`internal/boot/boot.go:1504`，调用点 L256/L629/L1069/L1086，confidence=0.90，exact-match，5 条边）；`NewSession`（`internal/agent/session.go:29`，调用点 L735/L1033/L1091，confidence=0.90，import，3 条边）；`subagentModelRef`（`internal/boot/boot.go:1327`，调用点 L715/L759/L840，confidence=0.90，exact-match，3 条边）；`subagentEffortRef`（`internal/boot/boot.go:1344`，调用点 L716/L760/L840，confidence=0.90，exact-match，3 条边）；`PluginSpecsForRootWithPlanModeAllowedTools`（`internal/boot/boot.go:1614`，调用点 L394/L426/L427，confidence=0.90，exact-match，3 条边）；`ArchiveDir`（`internal/config/paths.go:423`，调用点 L746/L827/L1102，confidence=0.90，import，3 条边）；`resolveSubagentProvider`（`internal/agent/parallel_tasks.go:540`，调用点 L718/L762，confidence=0.90，exact-match，2 条边）；`ReasoningLanguageFromContext`（`internal/agent/reasoning_language.go:170`，调用点 L748/L829，confidence=0.90，import，2 条边）
- **待源码复核被调候选（46）：** `Sink::Emit`（`internal/event/event.go:362`，调用点 L213/L220/L222/L231，confidence=0.80，instance-method，14 条边，待源码复核）；`Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L186/L613/L1065/L1081，confidence=0.70，instance-method，5 条边，待源码复核）；`Registry::RemovePrefix`（`internal/tool/tool.go:182`，调用点 L892/L900/L910/L1011，confidence=0.70，instance-method，5 条边，待源码复核）；`addTools`（`internal/boot/token_profile.go:251`，调用点 L556/L970/L1012/L1021，confidence=0.70，exact-match，4 条边，待源码复核）；`Host::ToolsFor`（`internal/plugin/plugin.go:712`，调用点 L450/L469/L508/L1007，confidence=0.70，instance-method，4 条边，待源码复核）；`Config::ReasoningLanguage`（`internal/config/config.go:396`，调用点 L1053/L1104/L1191，confidence=0.70，instance-method，3 条边，待源码复核）；`Config::WriteRootsForRoot`（`internal/config/config.go:781`，调用点 L361/L376/L972，confidence=0.70，instance-method，3 条边，待源码复核）；`SubagentStore::SaveFailed`（`internal/agent/subagent_store.go:553`，调用点 L832/L835，confidence=0.80，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** `App::ClearSession`（`desktop/app.go:1377`，调用点 L1389，confidence=0.70，instance-method，待源码复核）；`App::RefreshSkills`（`desktop/app.go:5127`，调用点 L5129，confidence=0.70，instance-method，待源码复核）；`App::SetModel`（`desktop/app.go:6021`，调用点 L6022，confidence=0.70，instance-method，待源码复核）；`App::SetEffort`（`desktop/app.go:6155`，调用点 L6156，confidence=0.70，instance-method，待源码复核）；`App::SetTokenMode`（`desktop/app.go:6236`，调用点 L6237，confidence=0.70，instance-method，待源码复核）；`App::CompleteOnboardingModelSelection`（`desktop/app.go:7820`，调用点 L7855，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 148 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 76 个文件；`internal/boot`、`desktop`、`internal/automation`、`internal/bot`、`internal/cli`、`internal/serve`、`internal/agent`、`internal/config`、`internal/expert`、`internal/skill`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/boot/boot_test.go`、`desktop/app_test.go`、`internal/boot/model_error_test.go`
- **同目录测试候选：** `internal/boot/helper_test.go`、`internal/boot/subagent_model_test.go`、`internal/boot/temphelper_test.go`
- **需同步检查：** `internal/boot/boot.go`、`desktop/app.go`、`internal/agent/session.go`、`internal/boot/boot_test.go`、`desktop/app_test.go`、`internal/boot/model_error_test.go`、`internal/boot/helper_test.go`

#### 待修改项

- [ ] 先冻结 `Build` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L154、L367、L560、L885、L1223 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Build` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-079、FN-011、ND-005 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/boot/boot.go|Build` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/boot/boot.go:152-1227` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Build` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G05 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-079`、`FN-011`、`ND-005` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/boot/boot_test.go`、`desktop/app_test.go`、`internal/boot/model_error_test.go`。
- [ ] 运行相关包测试：`go test ./internal/boot`。
- [ ] 运行相关包 race：`go test -race ./internal/boot`。
- [ ] 运行相关包 vet：`go vet ./internal/boot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/boot/boot.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/boot/boot.go|Build` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-011 · 函数有效行数 · `Build`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-011 |
| Baseline key | `func_lines\|internal/boot/boot.go\|Build` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P0（原始 P0，治理对象得分 83） |
| 文件 | `internal/boot/boot.go` |
| 代码行 | 152 |
| 函数/方法/包 | `Build` |
| Baseline / 当前复测 / 阈值 | 924 / 924 / 80 |
| 超出 | +844，11.55x |
| 同一治理对象关联问题 | `CX-079`、`ND-005` |

#### 明确的问题描述

`Build` 的有效代码行 **924**，超过限制 **80**（+844）。CodeGraph 定义物理范围为 152-1227 行，并记录 177 个直接下游候选；候选职责分布为 配置/解析（resolveSubagentProvider、ResolveSelection、TaskTool::WithTranscriptIdentityResolver、ResolveAgentMode）、状态与控制（RunSubAgentWithSession、EphemeralSubagentRun、FormatSubagentRunResult、BuildRunner）、持久化/I-O（ArchiveDir、CodeGraphReadTools、ReadOnlyIndexBlock、NewReadOnlyTaskTool）、渲染/输出（FormatSubagentRunResult、missingAPIKeyNotice、MCPStartupNotice、Render）、网络/协议（newProviderWithProxyForOwner、resolveSubagentProvider、NormalizeLegacyMimoCustomProvidersForRefs、NewBillableProviderAutoPlanClassifier）、权限/安全（PluginSpecsForRootWithPlanModeAllowedTools、rememberPermissionRuleForActor、applyPlanModeAllowedMCPToolTrust、planModeTrustedMCPServers）、任务/并发（IndexBlock、ReadOnlyIndexBlock、NewParallelTasksTool、NewTaskTool）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function Build（internal/boot/boot.go:152-1227）
- **可信生产调用方（11）：** `App::clearActiveSessionRuntime`（`desktop/app.go:1400`，调用点 L1429，confidence=0.90，import）；`App::SetModelForTab`（`desktop/app.go:6025`，调用点 L6097，confidence=0.90，import）；`App::SetEffortForTab`（`desktop/app.go:6159`，调用点 L6200，confidence=0.90，import）；`App::SetTokenModeForTab`（`desktop/app.go:6240`，调用点 L6276，confidence=0.90，import）；`App::rebuild`（`desktop/settings_app.go:901`，调用点 L932，confidence=0.90，import）；`App::buildTabControllerWithContext`（`desktop/tabs.go:2144`，调用点 L2275，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（131）：** `newProviderWithProxyForOwner`（`internal/boot/boot.go:1504`，调用点 L256/L629/L1069/L1086，confidence=0.90，exact-match，5 条边）；`NewSession`（`internal/agent/session.go:29`，调用点 L735/L1033/L1091，confidence=0.90，import，3 条边）；`subagentModelRef`（`internal/boot/boot.go:1327`，调用点 L715/L759/L840，confidence=0.90，exact-match，3 条边）；`subagentEffortRef`（`internal/boot/boot.go:1344`，调用点 L716/L760/L840，confidence=0.90，exact-match，3 条边）；`PluginSpecsForRootWithPlanModeAllowedTools`（`internal/boot/boot.go:1614`，调用点 L394/L426/L427，confidence=0.90，exact-match，3 条边）；`ArchiveDir`（`internal/config/paths.go:423`，调用点 L746/L827/L1102，confidence=0.90，import，3 条边）；`resolveSubagentProvider`（`internal/agent/parallel_tasks.go:540`，调用点 L718/L762，confidence=0.90，exact-match，2 条边）；`ReasoningLanguageFromContext`（`internal/agent/reasoning_language.go:170`，调用点 L748/L829，confidence=0.90，import，2 条边）
- **待源码复核被调候选（46）：** `Sink::Emit`（`internal/event/event.go:362`，调用点 L213/L220/L222/L231，confidence=0.80，instance-method，14 条边，待源码复核）；`Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L186/L613/L1065/L1081，confidence=0.70，instance-method，5 条边，待源码复核）；`Registry::RemovePrefix`（`internal/tool/tool.go:182`，调用点 L892/L900/L910/L1011，confidence=0.70，instance-method，5 条边，待源码复核）；`addTools`（`internal/boot/token_profile.go:251`，调用点 L556/L970/L1012/L1021，confidence=0.70，exact-match，4 条边，待源码复核）；`Host::ToolsFor`（`internal/plugin/plugin.go:712`，调用点 L450/L469/L508/L1007，confidence=0.70，instance-method，4 条边，待源码复核）；`Config::ReasoningLanguage`（`internal/config/config.go:396`，调用点 L1053/L1104/L1191，confidence=0.70，instance-method，3 条边，待源码复核）；`Config::WriteRootsForRoot`（`internal/config/config.go:781`，调用点 L361/L376/L972，confidence=0.70，instance-method，3 条边，待源码复核）；`SubagentStore::SaveFailed`（`internal/agent/subagent_store.go:553`，调用点 L832/L835，confidence=0.80，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** `App::ClearSession`（`desktop/app.go:1377`，调用点 L1389，confidence=0.70，instance-method，待源码复核）；`App::RefreshSkills`（`desktop/app.go:5127`，调用点 L5129，confidence=0.70，instance-method，待源码复核）；`App::SetModel`（`desktop/app.go:6021`，调用点 L6022，confidence=0.70，instance-method，待源码复核）；`App::SetEffort`（`desktop/app.go:6155`，调用点 L6156，confidence=0.70，instance-method，待源码复核）；`App::SetTokenMode`（`desktop/app.go:6236`，调用点 L6237，confidence=0.70，instance-method，待源码复核）；`App::CompleteOnboardingModelSelection`（`desktop/app.go:7820`，调用点 L7855，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 148 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 76 个文件；`internal/boot`、`desktop`、`internal/automation`、`internal/bot`、`internal/cli`、`internal/serve`、`internal/agent`、`internal/config`、`internal/expert`、`internal/skill`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/boot/boot_test.go`、`desktop/app_test.go`、`internal/boot/model_error_test.go`
- **同目录测试候选：** `internal/boot/helper_test.go`、`internal/boot/subagent_model_test.go`、`internal/boot/temphelper_test.go`
- **需同步检查：** `internal/boot/boot.go`、`desktop/app.go`、`internal/agent/session.go`、`internal/boot/boot_test.go`、`desktop/app_test.go`、`internal/boot/model_error_test.go`、`internal/boot/helper_test.go`

#### 待修改项

- [ ] 先冻结 `Build` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `Build` 的 924 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L162 `One-time import of v1/v0.5 legacy config — runs before Load so the freshly`、L466 `Race: another tab connected the same server between`、L684 `turn out wrong. The saved index loads into the prefix on the next session.`、L1205 `escalating to the human approval prompt.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-079、FN-011、ND-005 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/boot/boot.go|Build` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/boot/boot.go:152-1227` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Build` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G05 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-079`、`FN-011`、`ND-005` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/boot/boot_test.go`、`desktop/app_test.go`、`internal/boot/model_error_test.go`。
- [ ] 运行相关包测试：`go test ./internal/boot`。
- [ ] 运行相关包 race：`go test -race ./internal/boot`。
- [ ] 运行相关包 vet：`go vet ./internal/boot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/boot/boot.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/boot/boot.go|Build` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-005 · 嵌套深度 · `Build`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-005 |
| Baseline key | `nesting_depth\|internal/boot/boot.go\|Build` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P0（原始 P2，治理对象得分 83） |
| 文件 | `internal/boot/boot.go` |
| 代码行 | 152 |
| 函数/方法/包 | `Build` |
| Baseline / 当前复测 / 阈值 | 7 / 7 / 4 |
| 超出 | +3，1.75x |
| 同一治理对象关联问题 | `CX-079`、`FN-011` |

#### 明确的问题描述

`Build` 的最大嵌套深度为 **7**，超过限制 **4**。嵌套相关控制点包括：L154 `if stderr == nil {`；L330 `if err != nil {`；L471 `for _, t := range tools {`；L677 `if !tokenEconomy {`；L937 `for _, t := range skill.BuiltinSubagentTools(skillStore, skillRunner, skillProfile) {`；L1223 `if classifier != nil {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function Build（internal/boot/boot.go:152-1227）
- **可信生产调用方（11）：** `App::clearActiveSessionRuntime`（`desktop/app.go:1400`，调用点 L1429，confidence=0.90，import）；`App::SetModelForTab`（`desktop/app.go:6025`，调用点 L6097，confidence=0.90，import）；`App::SetEffortForTab`（`desktop/app.go:6159`，调用点 L6200，confidence=0.90，import）；`App::SetTokenModeForTab`（`desktop/app.go:6240`，调用点 L6276，confidence=0.90，import）；`App::rebuild`（`desktop/settings_app.go:901`，调用点 L932，confidence=0.90，import）；`App::buildTabControllerWithContext`（`desktop/tabs.go:2144`，调用点 L2275，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（131）：** `newProviderWithProxyForOwner`（`internal/boot/boot.go:1504`，调用点 L256/L629/L1069/L1086，confidence=0.90，exact-match，5 条边）；`NewSession`（`internal/agent/session.go:29`，调用点 L735/L1033/L1091，confidence=0.90，import，3 条边）；`subagentModelRef`（`internal/boot/boot.go:1327`，调用点 L715/L759/L840，confidence=0.90，exact-match，3 条边）；`subagentEffortRef`（`internal/boot/boot.go:1344`，调用点 L716/L760/L840，confidence=0.90，exact-match，3 条边）；`PluginSpecsForRootWithPlanModeAllowedTools`（`internal/boot/boot.go:1614`，调用点 L394/L426/L427，confidence=0.90，exact-match，3 条边）；`ArchiveDir`（`internal/config/paths.go:423`，调用点 L746/L827/L1102，confidence=0.90，import，3 条边）；`resolveSubagentProvider`（`internal/agent/parallel_tasks.go:540`，调用点 L718/L762，confidence=0.90，exact-match，2 条边）；`ReasoningLanguageFromContext`（`internal/agent/reasoning_language.go:170`，调用点 L748/L829，confidence=0.90，import，2 条边）
- **待源码复核被调候选（46）：** `Sink::Emit`（`internal/event/event.go:362`，调用点 L213/L220/L222/L231，confidence=0.80，instance-method，14 条边，待源码复核）；`Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L186/L613/L1065/L1081，confidence=0.70，instance-method，5 条边，待源码复核）；`Registry::RemovePrefix`（`internal/tool/tool.go:182`，调用点 L892/L900/L910/L1011，confidence=0.70，instance-method，5 条边，待源码复核）；`addTools`（`internal/boot/token_profile.go:251`，调用点 L556/L970/L1012/L1021，confidence=0.70，exact-match，4 条边，待源码复核）；`Host::ToolsFor`（`internal/plugin/plugin.go:712`，调用点 L450/L469/L508/L1007，confidence=0.70，instance-method，4 条边，待源码复核）；`Config::ReasoningLanguage`（`internal/config/config.go:396`，调用点 L1053/L1104/L1191，confidence=0.70，instance-method，3 条边，待源码复核）；`Config::WriteRootsForRoot`（`internal/config/config.go:781`，调用点 L361/L376/L972，confidence=0.70，instance-method，3 条边，待源码复核）；`SubagentStore::SaveFailed`（`internal/agent/subagent_store.go:553`，调用点 L832/L835，confidence=0.80，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** `App::ClearSession`（`desktop/app.go:1377`，调用点 L1389，confidence=0.70，instance-method，待源码复核）；`App::RefreshSkills`（`desktop/app.go:5127`，调用点 L5129，confidence=0.70，instance-method，待源码复核）；`App::SetModel`（`desktop/app.go:6021`，调用点 L6022，confidence=0.70，instance-method，待源码复核）；`App::SetEffort`（`desktop/app.go:6155`，调用点 L6156，confidence=0.70，instance-method，待源码复核）；`App::SetTokenMode`（`desktop/app.go:6236`，调用点 L6237，confidence=0.70，instance-method，待源码复核）；`App::CompleteOnboardingModelSelection`（`desktop/app.go:7820`，调用点 L7855，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 148 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 76 个文件；`internal/boot`、`desktop`、`internal/automation`、`internal/bot`、`internal/cli`、`internal/serve`、`internal/agent`、`internal/config`、`internal/expert`、`internal/skill`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/boot/boot_test.go`、`desktop/app_test.go`、`internal/boot/model_error_test.go`
- **同目录测试候选：** `internal/boot/helper_test.go`、`internal/boot/subagent_model_test.go`、`internal/boot/temphelper_test.go`
- **需同步检查：** `internal/boot/boot.go`、`desktop/app.go`、`internal/agent/session.go`、`internal/boot/boot_test.go`、`desktop/app_test.go`、`internal/boot/model_error_test.go`、`internal/boot/helper_test.go`

#### 待修改项

- [ ] 先冻结 `Build` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 从 L154、L367、L560、L885、L1223 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-079、FN-011、ND-005 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/boot/boot.go|Build` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/boot/boot.go:152-1227` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Build` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G05 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-079`、`FN-011`、`ND-005` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/boot/boot_test.go`、`desktop/app_test.go`、`internal/boot/model_error_test.go`。
- [ ] 运行相关包测试：`go test ./internal/boot`。
- [ ] 运行相关包 race：`go test -race ./internal/boot`。
- [ ] 运行相关包 vet：`go vet ./internal/boot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/boot/boot.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/boot/boot.go|Build` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-032 · 圈复杂度 · `migrateLegacySessions`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-032 |
| Baseline key | `complexity\|internal/agent/migrate.go\|migrateLegacySessions` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 69） |
| 文件 | `internal/agent/migrate.go` |
| 代码行 | 82 |
| 函数/方法/包 | `migrateLegacySessions` |
| Baseline / 当前复测 / 阈值 | 54 / 54 / 10 |
| 超出 | +44，5.40x |
| 同一治理对象关联问题 | `FN-006`、`ND-003` |

#### 明确的问题描述

`migrateLegacySessions` 的圈复杂度当前为 **54**，超过限制 **10**（+44，5.40x）。定义范围为 82-266 行；可见控制流热点：L83 `if strings.TrimSpace(marker) == "" {`；L127 `if projectDir != nil && meta.Workspace != "" && dirExists(meta.Workspace) {`；L161 `if err != nil || len(msgs) == 0 {`；L196 `if _, err := os.Stat(filepath.Join(srcDir, jsonlName)); err == nil {`；L230 `if !e.IsDir() {`；L261 `if hadArtifactFailure {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readLegacyMeta、migrateSubDirectory、writeImportMarkers）、渲染/输出（isMessageFormat）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function migrateLegacySessions（internal/agent/migrate.go:82-266）
- **可信生产调用方（2）：** `MigrateLegacySessions`（`internal/agent/migrate.go:70`，调用点 L71，confidence=0.90，exact-match）；`MigrateLegacySessionsFromConfigDir`（`internal/agent/migrate.go:78`，调用点 L79，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（11）：** `recordImportedTitle`（`internal/agent/migrate.go:613`，调用点 L138/L153/L171/L221，confidence=0.90，exact-match，4 条边）；`importMarkerExists`（`internal/agent/migrate.go:638`，调用点 L89/L180，confidence=0.90，exact-match，3 条边）；`isMessageFormat`（`internal/agent/migrate.go:444`，调用点 L147/L211，confidence=0.90，exact-match，2 条边）；`transformAndCopyJsonl`（`internal/agent/migrate.go:482`，调用点 L149/L215，confidence=0.90，exact-match，2 条边）；`readLegacyMeta`（`internal/agent/migrate.go:572`，调用点 L125/L199，confidence=0.90，exact-match，2 条边）；`importJsonlSessions`（`internal/agent/migrate.go:271`，调用点 L181，confidence=0.90，exact-match）；`migrateSubDirectory`（`internal/agent/migrate.go:352`，调用点 L252，confidence=0.90，exact-match）；`moveFlatImport`（`internal/agent/migrate.go:592`，调用点 L137，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `dirExists`（`internal/agent/migrate.go:584`，调用点 L127/L201，confidence=0.70，exact-match，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** `MigrateLegacySessions`（`internal/agent/migrate.go:70`，调用点 L71，confidence=0.90，exact-match，待源码复核）；`MigrateLegacySessionsFromConfigDir`（`internal/agent/migrate.go:78`，调用点 L79，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/migrate_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/migrate.go`、`internal/agent/migrate_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`

#### 待修改项

- [ ] 先冻结 `migrateLegacySessions` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L83、L133、L186、L218、L261 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `migrateLegacySessions` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-032、FN-006、ND-003 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/migrate.go|migrateLegacySessions` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/migrate.go:82-266` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `migrateLegacySessions` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G05 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-032`、`FN-006`、`ND-003` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/migrate_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/migrate.go|migrateLegacySessions` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-006 · 函数有效行数 · `migrateLegacySessions`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-006 |
| Baseline key | `func_lines\|internal/agent/migrate.go\|migrateLegacySessions` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P2，治理对象得分 69） |
| 文件 | `internal/agent/migrate.go` |
| 代码行 | 82 |
| 函数/方法/包 | `migrateLegacySessions` |
| Baseline / 当前复测 / 阈值 | 146 / 146 / 80 |
| 超出 | +66，1.82x |
| 同一治理对象关联问题 | `CX-032`、`ND-003` |

#### 明确的问题描述

`migrateLegacySessions` 的有效代码行 **146**，超过限制 **80**（+66）。CodeGraph 定义物理范围为 82-266 行，并记录 12 个直接下游候选；候选职责分布为 持久化/I-O（readLegacyMeta、migrateSubDirectory、writeImportMarkers）、渲染/输出（isMessageFormat）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function migrateLegacySessions（internal/agent/migrate.go:82-266）
- **可信生产调用方（2）：** `MigrateLegacySessions`（`internal/agent/migrate.go:70`，调用点 L71，confidence=0.90，exact-match）；`MigrateLegacySessionsFromConfigDir`（`internal/agent/migrate.go:78`，调用点 L79，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（11）：** `recordImportedTitle`（`internal/agent/migrate.go:613`，调用点 L138/L153/L171/L221，confidence=0.90，exact-match，4 条边）；`importMarkerExists`（`internal/agent/migrate.go:638`，调用点 L89/L180，confidence=0.90，exact-match，3 条边）；`isMessageFormat`（`internal/agent/migrate.go:444`，调用点 L147/L211，confidence=0.90，exact-match，2 条边）；`transformAndCopyJsonl`（`internal/agent/migrate.go:482`，调用点 L149/L215，confidence=0.90，exact-match，2 条边）；`readLegacyMeta`（`internal/agent/migrate.go:572`，调用点 L125/L199，confidence=0.90，exact-match，2 条边）；`importJsonlSessions`（`internal/agent/migrate.go:271`，调用点 L181，confidence=0.90，exact-match）；`migrateSubDirectory`（`internal/agent/migrate.go:352`，调用点 L252，confidence=0.90，exact-match）；`moveFlatImport`（`internal/agent/migrate.go:592`，调用点 L137，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `dirExists`（`internal/agent/migrate.go:584`，调用点 L127/L201，confidence=0.70，exact-match，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** `MigrateLegacySessions`（`internal/agent/migrate.go:70`，调用点 L71，confidence=0.90，exact-match，待源码复核）；`MigrateLegacySessionsFromConfigDir`（`internal/agent/migrate.go:78`，调用点 L79，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/migrate_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/migrate.go`、`internal/agent/migrate_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`

#### 待修改项

- [ ] 先冻结 `migrateLegacySessions` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `migrateLegacySessions` 的 146 个有效行按 持久化/I-O、渲染/输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L86 `Gate on both the routed marker AND the jsonl marker: an existing upgrader`、L103 `Build the set of base names that have a .events.jsonl so the .jsonl-only`、L176 `These are sessions the TS version wrote directly in the v1+ format (ACP,`、L260 `re-run the flat import over routed sessions.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-032、FN-006、ND-003 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/agent/migrate.go|migrateLegacySessions` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/migrate.go:82-266` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `migrateLegacySessions` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G05 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-032`、`FN-006`、`ND-003` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/migrate_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/agent/migrate.go|migrateLegacySessions` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-003 · 嵌套深度 · `migrateLegacySessions`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-003 |
| Baseline key | `nesting_depth\|internal/agent/migrate.go\|migrateLegacySessions` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P3，治理对象得分 69） |
| 文件 | `internal/agent/migrate.go` |
| 代码行 | 82 |
| 函数/方法/包 | `migrateLegacySessions` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-032`、`FN-006` |

#### 明确的问题描述

`migrateLegacySessions` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L83 `if strings.TrimSpace(marker) == "" {`；L127 `if projectDir != nil && meta.Workspace != "" && dirExists(meta.Workspace) {`；L161 `if err != nil || len(msgs) == 0 {`；L196 `if _, err := os.Stat(filepath.Join(srcDir, jsonlName)); err == nil {`；L230 `if !e.IsDir() {`；L261 `if hadArtifactFailure {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function migrateLegacySessions（internal/agent/migrate.go:82-266）
- **可信生产调用方（2）：** `MigrateLegacySessions`（`internal/agent/migrate.go:70`，调用点 L71，confidence=0.90，exact-match）；`MigrateLegacySessionsFromConfigDir`（`internal/agent/migrate.go:78`，调用点 L79，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（11）：** `recordImportedTitle`（`internal/agent/migrate.go:613`，调用点 L138/L153/L171/L221，confidence=0.90，exact-match，4 条边）；`importMarkerExists`（`internal/agent/migrate.go:638`，调用点 L89/L180，confidence=0.90，exact-match，3 条边）；`isMessageFormat`（`internal/agent/migrate.go:444`，调用点 L147/L211，confidence=0.90，exact-match，2 条边）；`transformAndCopyJsonl`（`internal/agent/migrate.go:482`，调用点 L149/L215，confidence=0.90，exact-match，2 条边）；`readLegacyMeta`（`internal/agent/migrate.go:572`，调用点 L125/L199，confidence=0.90，exact-match，2 条边）；`importJsonlSessions`（`internal/agent/migrate.go:271`，调用点 L181，confidence=0.90，exact-match）；`migrateSubDirectory`（`internal/agent/migrate.go:352`，调用点 L252，confidence=0.90，exact-match）；`moveFlatImport`（`internal/agent/migrate.go:592`，调用点 L137，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `dirExists`（`internal/agent/migrate.go:584`，调用点 L127/L201，confidence=0.70，exact-match，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** `MigrateLegacySessions`（`internal/agent/migrate.go:70`，调用点 L71，confidence=0.90，exact-match，待源码复核）；`MigrateLegacySessionsFromConfigDir`（`internal/agent/migrate.go:78`，调用点 L79，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/migrate_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/migrate.go`、`internal/agent/migrate_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`

#### 待修改项

- [ ] 先冻结 `migrateLegacySessions` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 从 L83、L133、L186、L218、L261 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-032、FN-006、ND-003 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/agent/migrate.go|migrateLegacySessions` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/migrate.go:82-266` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `migrateLegacySessions` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G05 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-032`、`FN-006`、`ND-003` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/migrate_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/agent/migrate.go|migrateLegacySessions` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FL-004 · 文件有效行数 · `internal/boot/boot.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-004 |
| Baseline key | `file_lines\|internal/boot/boot.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 51） |
| 文件 | `internal/boot/boot.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/boot/boot.go` |
| Baseline / 当前复测 / 阈值 | 1481 / 1481 / 800 |
| 超出 | +681，1.85x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/boot/boot.go` 当前有效代码行 **1481**，超过限制 **800**（+681）。CodeGraph 在文件中索引到的主要大符号包括 Build@152-1227、subagentEffectiveIdentity@1454-1489、addBuiltins@1533-1567、LSPSpecs@1747-1775、rememberPermissionRuleForActor@1248-1275、subagentModelKeys@1379-1404；文件同时承载的调用影响覆盖 25 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 40 个函数/方法节点。
- **可信生产调用方（7）：** `App::clearActiveSessionRuntime`（`desktop/app.go:1400`，调用点 L1429，confidence=0.90，import）；`App::SetModelForTab`（`desktop/app.go:6025`，调用点 L6097，confidence=0.90，import）；`App::SetEffortForTab`（`desktop/app.go:6159`，调用点 L6200，confidence=0.90，import）；`App::SetTokenModeForTab`（`desktop/app.go:6240`，调用点 L6276，confidence=0.90，import）；`App::rebuild`（`desktop/settings_app.go:901`，调用点 L932，confidence=0.90，import）；`App::buildTabControllerWithContext`（`desktop/tabs.go:2144`，调用点 L2275，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（24）：** `NewSession`（`internal/agent/session.go:29`，调用点 L735/L1033/L1091，confidence=0.90，import，3 条边）；`ArchiveDir`（`internal/config/paths.go:423`，调用点 L746/L827/L1102，confidence=0.90，import，3 条边）；`resolveSubagentProvider`（`internal/agent/parallel_tasks.go:540`，调用点 L718/L762，confidence=0.90，exact-match，2 条边）；`ReasoningLanguageFromContext`（`internal/agent/reasoning_language.go:170`，调用点 L748/L829，confidence=0.90，import，2 条边）；`RunSubAgentWithSession`（`internal/agent/task.go:804`，调用点 L735/L819，confidence=0.90，import，2 条边）；`NestedSink`（`internal/agent/task.go:828`，调用点 L749/L830，confidence=0.90，import，2 条边）；`NormalizeEffort`（`internal/config/effort.go:86`，调用点 L620/L1473，confidence=0.90，import，2 条边）；`EffectiveEffort`（`internal/config/effort.go:193`，调用点 L1488/L1518，confidence=0.90，import，2 条边）
- **待源码复核被调候选（16）：** `Sink::Emit`（`internal/event/event.go:362`，调用点 L213/L220/L222/L231，confidence=0.80，instance-method，14 条边，待源码复核）；`Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L186/L613/L1065/L1081，confidence=0.70，instance-method，7 条边，待源码复核）；`Registry::RemovePrefix`（`internal/tool/tool.go:182`，调用点 L892/L900/L910/L1011，confidence=0.70，instance-method，5 条边，待源码复核）；`addTools`（`internal/boot/token_profile.go:251`，调用点 L556/L970/L1012/L1021，confidence=0.70，exact-match，4 条边，待源码复核）；`ProviderEntry::APIKey`（`internal/config/config.go:1390`，调用点 L230/L1182/L1509/L1307，confidence=0.70，instance-method，4 条边，待源码复核）；`Host::ToolsFor`（`internal/plugin/plugin.go:712`，调用点 L450/L469/L508/L1007，confidence=0.70，instance-method，4 条边，待源码复核）；`Config::WriteRootsForRoot`（`internal/config/config.go:781`，调用点 L361/L376/L972，confidence=0.70，instance-method，3 条边，待源码复核）；`Config::ReasoningLanguage`（`internal/config/config.go:396`，调用点 L1053/L1104/L1191，confidence=0.70，instance-method，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 253 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 25 个文件；`internal/boot`、`desktop`、`internal/automation`、`internal/event`、`internal/config`、`internal/tool`、`internal/plugin`、`internal/agent`、`internal/expert`、`internal/permission`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/boot/boot_test.go`、`desktop/app_test.go`
- **同目录测试候选：** `internal/boot/helper_test.go`、`internal/boot/model_error_test.go`、`internal/boot/subagent_model_test.go`、`internal/boot/temphelper_test.go`
- **需同步检查：** `internal/boot/boot.go`、`desktop/app.go`、`internal/event/event.go`、`internal/config/config.go`、`internal/tool/tool.go`、`internal/boot/token_profile.go`、`internal/boot/boot_test.go`、`desktop/app_test.go`、`internal/boot/helper_test.go`、`internal/boot/model_error_test.go`

#### 待修改项

- [ ] 先冻结 `internal/boot/boot.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `Build`、`subagentEffectiveIdentity`、`addBuiltins`、`LSPSpecs`、`rememberPermissionRuleForActor` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-079、CX-080、FN-011、ND-005。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/boot/boot.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/boot/boot.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/boot/boot.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G05 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/boot/boot_test.go`、`desktop/app_test.go`。
- [ ] 运行相关包测试：`go test ./internal/boot`。
- [ ] 运行相关包 race：`go test -race ./internal/boot`。
- [ ] 运行相关包 vet：`go vet ./internal/boot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/boot/boot.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/boot/boot.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-037 · 圈复杂度 · `rehomeStrandedSessions`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-037 |
| Baseline key | `complexity\|internal/agent/migrate.go\|rehomeStrandedSessions` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 48） |
| 文件 | `internal/agent/migrate.go` |
| 代码行 | 685 |
| 函数/方法/包 | `rehomeStrandedSessions` |
| Baseline / 当前复测 / 阈值 | 20 / 20 / 10 |
| 超出 | +10，2.00x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`rehomeStrandedSessions` 的圈复杂度当前为 **20**，超过限制 **10**（+10，2.00x）。定义范围为 685-750 行；可见控制流热点：L686 `if projectDir == nil {`；L702 `for _, e := range entries {`；L709 `if strings.HasPrefix(base, "subagent-") {`；L721 `if destDir == "" || sameDirPath(destDir, globalDest) {`；L726 `if err := copySubagentArtifacts(srcDir, destDir, base); err != nil {`；L745 `if !hadCopyFailure {`。CodeGraph 记录的下游职责候选主要涉及 渲染/输出（isMessageFormat）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function rehomeStrandedSessions（internal/agent/migrate.go:685-750）
- **可信生产调用方（1）：** `migrateLegacySessions`（`internal/agent/migrate.go:82`，调用点 L96，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（7）：** `copySubagentArtifacts`（`internal/agent/migrate.go:810`，调用点 L726/L737，confidence=0.90，exact-match，2 条边）；`isMessageFormat`（`internal/agent/migrate.go:444`，调用点 L717，confidence=0.90，exact-match）；`transformAndCopyJsonl`（`internal/agent/migrate.go:482`，调用点 L731，confidence=0.90，exact-match）；`recordImportedTitle`（`internal/agent/migrate.go:613`，调用点 L740，confidence=0.90，exact-match）；`strandedSessionDestDir`（`internal/agent/migrate.go:756`，调用点 L720，confidence=0.90，exact-match）；`copyBranchMetaSidecar`（`internal/agent/migrate.go:782`，调用点 L736，confidence=0.90，exact-match）；`sameDirPath`（`internal/agent/migrate.go:876`，调用点 L721，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `MigrateLegacySessions`（`internal/agent/migrate.go:70`，调用点 L71，confidence=0.90，exact-match，待源码复核）；`MigrateLegacySessionsFromConfigDir`（`internal/agent/migrate.go:78`，调用点 L79，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/migrate.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `rehomeStrandedSessions` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L686、L702、L717、L726、L745 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `rehomeStrandedSessions` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/migrate.go|rehomeStrandedSessions` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/migrate.go:685-750` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `rehomeStrandedSessions` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G05 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/migrate.go|rehomeStrandedSessions` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-081 · 圈复杂度 · `*toolSourceConnector.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-081 |
| Baseline key | `complexity\|internal/boot/token_profile.go\|*toolSourceConnector.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 43） |
| 文件 | `internal/boot/token_profile.go` |
| 代码行 | 108 |
| 函数/方法/包 | `*toolSourceConnector.Execute` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*toolSourceConnector.Execute` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 108-160 行；可见控制流热点：L113 `if err := json.Unmarshal(args, &p); err != nil {`；L129 `switch source {`；L134 `case "task":`；L138 `case "install_source":`；L144 `case "mcp":`；L153 `if t.mcp == nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeToolSource）、状态与控制（runSourceInstaller）、任务/并发（toolSourceConnector::planModeSourceBlocked）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method toolSourceConnector::Execute（internal/boot/token_profile.go:108-160）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `runSourceInstaller`（`internal/boot/token_profile.go:244`，调用点 L131/L133/L135/L137，confidence=0.90，exact-match，7 条边）；`normalizeToolSource`（`internal/boot/token_profile.go:191`，调用点 L116，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `toolSourceConnector::planModeSourceBlocked`（`internal/boot/token_profile.go:162`，调用点 L125，confidence=0.70，instance-method，待源码复核）；`toolSourceConnector::availableSources`（`internal/boot/token_profile.go:214`，调用点 L118，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/boot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/boot/boot_test.go`、`internal/boot/helper_test.go`、`internal/boot/model_error_test.go`、`internal/boot/subagent_model_test.go`、`internal/boot/temphelper_test.go`
- **需同步检查：** `internal/boot/token_profile.go`、`internal/boot/boot_test.go`、`internal/boot/helper_test.go`、`internal/boot/model_error_test.go`、`internal/boot/subagent_model_test.go`

#### 待修改项

- [ ] 先冻结 `*toolSourceConnector.Execute` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L113、L130、L136、L144、L153 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*toolSourceConnector.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/boot/token_profile.go|*toolSourceConnector.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/boot/token_profile.go:108-160` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*toolSourceConnector.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G05 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/boot/boot_test.go`、`internal/boot/helper_test.go`、`internal/boot/model_error_test.go`、`internal/boot/subagent_model_test.go`、`internal/boot/temphelper_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/boot`。
- [ ] 运行相关包 vet：`go vet ./internal/boot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/boot/token_profile.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/boot/token_profile.go|*toolSourceConnector.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-080 · 圈复杂度 · `subagentEffectiveIdentity`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-080 |
| Baseline key | `complexity\|internal/boot/boot.go\|subagentEffectiveIdentity` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/boot/boot.go` |
| 代码行 | 1454 |
| 函数/方法/包 | `subagentEffectiveIdentity` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`subagentEffectiveIdentity` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 1454-1489 行；可见控制流热点：L1456 `if base != nil {`；L1460 `if ref == "" {`；L1463 `if cfg != nil && ref != "" {`；L1472 `if rawEffort := strings.TrimSpace(effort); rawEffort != "" {`；L1473 `if normalized, err := config.NormalizeEffort(&entry, rawEffort); err == nil {`；L1481 `if modelID != "" && model != "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（NormalizeEffort、Config::ResolveModel）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function subagentEffectiveIdentity（internal/boot/boot.go:1454-1489）
- **可信生产调用方（1）：** `Build`（`internal/boot/boot.go:152`，调用点 L636，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `NormalizeEffort`（`internal/config/effort.go:86`，调用点 L1473，confidence=0.90，import）；`EffectiveEffort`（`internal/config/effort.go:193`，调用点 L1488，confidence=0.90，import）
- **待源码复核被调候选（1）：** `Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L1464，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `buildController`（`internal/automation/runner.go:65`，调用点 L66，confidence=0.90，import，待源码复核）；`Build`（`internal/boot/boot.go:152`，调用点 L636，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 9 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/boot`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/boot/boot_test.go`、`internal/boot/helper_test.go`、`internal/boot/model_error_test.go`、`internal/boot/subagent_model_test.go`、`internal/boot/temphelper_test.go`
- **需同步检查：** `internal/boot/boot.go`、`internal/config/effort.go`、`internal/config/config.go`、`internal/boot/boot_test.go`、`internal/boot/helper_test.go`、`internal/boot/model_error_test.go`、`internal/boot/subagent_model_test.go`

#### 待修改项

- [ ] 先冻结 `subagentEffectiveIdentity` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1456、L1463、L1464、L1473、L1481 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `subagentEffectiveIdentity` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/boot/boot.go|subagentEffectiveIdentity` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/boot/boot.go:1454-1489` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `subagentEffectiveIdentity` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G05 处理。
- 重点保护：配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/boot/boot_test.go`、`internal/boot/helper_test.go`、`internal/boot/model_error_test.go`、`internal/boot/subagent_model_test.go`、`internal/boot/temphelper_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/boot`。
- [ ] 运行相关包 vet：`go vet ./internal/boot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/boot/boot.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/boot/boot.go|subagentEffectiveIdentity` 下降或消失且无新增/恶化。
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
