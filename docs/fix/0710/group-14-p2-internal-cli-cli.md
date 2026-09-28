# G14 · internal/cli · cli 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G14 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/cli` |
| 涉及文件 | `internal/cli`<br>`internal/cli/cli.go` |
| 主要治理目标 | 围绕 `internal/cli/cli.go` 等治理对象降低复杂度/体量，统一处理 3 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | cli 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614`、`origin/expert-202607081622`；单文件最多集中 13 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 14 组；先于 G42 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容；并发、取消或事件顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **53/100**，对象 `internal/cli` 的分项为严重度 16.6、集中度 2.3、影响面 14.0、可执行性 20.0；最高超限 1.87x，文件内 1 条问题，CodeGraph 记录 2 个可信生产调用方、4 个待核候选。

### 治理边界与组间关系

本组涉及 2 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G42，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `PF-001` | `package_files\|internal/cli\|` | 单包生产文件数 | `internal/cli` | `internal/cli` | 1 | 56/30 | +26（1.87x） | P2 | 2 可信 + 4 待核 caller / 33 可信 + 7 待核 callee / 3147 unresolved | 待修改 |
| `CX-123` | `complexity\|internal/cli/cli.go\|chatREPL` | 圈复杂度 | `internal/cli/cli.go` | `chatREPL` | 652 | 37/10 | +27（3.70x） | P2 | 0 可信 + 0 待核 caller / 21 可信 + 7 待核 callee / 41 unresolved | 待修改 |
| `FN-024` | `func_lines\|internal/cli/cli.go\|chatREPL` | 函数有效行数 | `internal/cli/cli.go` | `chatREPL` | 652 | 148/80 | +68（1.85x） | P2 | 0 可信 + 0 待核 caller / 21 可信 + 7 待核 callee / 41 unresolved | 待修改 |
| `CX-121` | `complexity\|internal/cli/cli.go\|runAgent` | 圈复杂度 | `internal/cli/cli.go` | `runAgent` | 387 | 26/10 | +16（2.60x） | P2 | 1 可信 + 0 待核 caller / 18 可信 + 2 待核 callee / 36 unresolved | 待修改 |
| `FN-022` | `func_lines\|internal/cli/cli.go\|runAgent` | 函数有效行数 | `internal/cli/cli.go` | `runAgent` | 387 | 101/80 | +21（1.26x） | P2 | 1 可信 + 0 待核 caller / 18 可信 + 2 待核 callee / 36 unresolved | 待修改 |
| `FL-007` | `file_lines\|internal/cli/cli.go\|` | 文件有效行数 | `internal/cli/cli.go` | `internal/cli/cli.go` | 1 | 1638/800 | +838（2.05x） | P2 | 4 可信 + 0 待核 caller / 27 可信 + 13 待核 callee / 422 unresolved | 待修改 |
| `CX-122` | `complexity\|internal/cli/cli.go\|runServe` | 圈复杂度 | `internal/cli/cli.go` | `runServe` | 508 | 24/10 | +14（2.40x） | P2 | 1 可信 + 0 待核 caller / 12 可信 + 6 待核 callee / 39 unresolved | 待修改 |
| `FN-023` | `func_lines\|internal/cli/cli.go\|runServe` | 函数有效行数 | `internal/cli/cli.go` | `runServe` | 508 | 110/80 | +30（1.38x） | P2 | 1 可信 + 0 待核 caller / 12 可信 + 6 待核 callee / 39 unresolved | 待修改 |
| `CX-120` | `complexity\|internal/cli/cli.go\|Run` | 圈复杂度 | `internal/cli/cli.go` | `Run` | 51 | 27/10 | +17（2.70x） | P3 | 0 可信 + 0 待核 caller / 21 可信 + 1 待核 callee / 6 unresolved | 待修改 |
| `CX-124` | `complexity\|internal/cli/cli.go\|selectEnabledProviders` | 圈复杂度 | `internal/cli/cli.go` | `selectEnabledProviders` | 1120 | 15/10 | +5（1.50x） | P3 | 1 可信 + 0 待核 caller / 11 可信 + 0 待核 callee / 26 unresolved | 待修改 |

## 3. 逐条治理任务

### PF-001 · 单包生产文件数 · `internal/cli`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | PF-001 |
| Baseline key | `package_files\|internal/cli\|` |
| 规则 | package_files（单包生产文件数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 53） |
| 文件 | `internal/cli` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/cli` |
| Baseline / 当前复测 / 阈值 | 56 / 56 / 30 |
| 超出 | +26，1.87x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/cli` 包当前生产文件数 **56**，超过限制 **30**（+26）。CodeGraph 当前映射摘要：CodeGraph 在 internal/cli 下索引到 100 个 Go 文件、1195 个函数/方法节点。 示例文件：`internal/cli/acp.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/autoplan.go`、`internal/cli/bot.go`、`internal/cli/bot_test.go`、`internal/cli/box.go`、`internal/cli/box_test.go`、`internal/cli/branch.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`。该规则只能证明文件数量超限，后续需要区分合理边界与无意义碎片。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在 internal/cli 下索引到 100 个 Go 文件、1195 个函数/方法节点。
- **可信生产调用方（2）：** `colorEnabled`（`internal/cli/style.go:12`，调用点 L12，confidence=0.90，exact-match）；`activeCLITheme`（`internal/cli/theme.go:96`，调用点 L96，confidence=0.90，exact-match）
- **待源码复核调用方候选（4）：** `App`（`desktop/frontend/src/App.tsx:840`，调用点 L3824，confidence=0.80，framework，待源码复核）；`IsNil`（`internal/nilutil/nil.go:6`，调用点 L11，confidence=0.70，instance-method，待源码复核）；`run`（`scripts/code_guard/main.go:76`，调用点 L100，confidence=0.70，instance-method，待源码复核）；`codeLineSet`（`scripts/code_guard/main.go:565`，调用点 L570，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（33）：** `New`（`internal/control/controller.go:332`，调用点 L632/L457/L687/L248，confidence=0.90，import，68 条边）；`UserConfigPath`（`internal/config/paths.go:380`，调用点 L106/L158/L161/L23，confidence=0.90，import，52 条边）；`NewProfilePathResolver`（`internal/config/paths.go:81`，调用点 L173/L68/L34/L125，confidence=0.90，import，46 条边）；`DetectLanguage`（`internal/i18n/i18n.go:444`，调用点 L55/L72/L1001/L22，confidence=0.90，import，39 条边）；`Default`（`internal/config/config.go:1230`，调用点 L126/L233/L242/L195，confidence=0.90，import，33 条边）；`xlsxCell::Value`（`internal/automation/business.go:1089`，调用点 L867/L1001/L1049/L1085，confidence=0.90，exact-match，32 条边）；`LoadForEdit`（`internal/config/load.go:392`，调用点 L2021/L991/L1961/L1914，confidence=0.90，import，13 条边）；`New`（`internal/agent/agent.go:603`，调用点 L455/L171/L51/L394，confidence=0.90，import，11 条边）
- **待源码复核被调候选（7）：** `ProfilePathResolver::UserConfigPath`（`internal/config/paths.go:176`，调用点 L50/L35/L38/L128，confidence=0.85，instance-method，53 条边，待源码复核）；`Config::SaveToScope`（`internal/config/edit.go:820`，调用点 L71/L2030/L128/L87，confidence=0.70，instance-method，25 条边，待源码复核）；`Config::SaveTo`（`internal/config/edit.go:809`，调用点 L2028/L1029/L23/L720，confidence=0.70，instance-method，22 条边，待源码复核）；`Config::ReasoningLanguage`（`internal/config/config.go:396`，调用点 L1991/L2036/L751/L752，confidence=0.70，instance-method，6 条边，待源码复核）；`Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L802/L50/L1718/L321，confidence=0.70，instance-method，4 条边，待源码复核）；`ProfilePathResolver::UserCredentialsPath`（`internal/config/paths.go:195`，调用点 L64/L65/L72/L73，confidence=0.70，instance-method，4 条边，待源码复核）；`Config::MemoryCompilerEnabled`（`internal/config/config.go:883`，调用点 L1948/L1970/L628，confidence=0.70，instance-method，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 3147 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 123 个文件；`internal/cli`、`desktop`、`internal/nilutil`、`scripts`、`internal/control`、`internal/config`、`internal/i18n`、`internal/automation`、`internal/agent`、`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/automation/model_test.go`、`desktop/bound_array_contract_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`、`internal/cli/complete_test.go`、`internal/cli/consecutive_tool_markers_test.go`、`internal/cli/diffview_test.go`、`internal/cli/doctor_test.go`、`internal/cli/gitstatus_test.go`、`internal/cli/interject_test.go`、`internal/cli/latex_test.go`、`internal/cli/math_e2e_test.go`、`internal/cli/mcp_test.go`、`internal/cli/md_cjk_test.go`、`internal/cli/md_test.go`、`internal/cli/model_test.go`、`internal/cli/profile_entry_test.go`、`internal/cli/rename_test.go`、`internal/cli/render_edge_test.go`、`internal/cli/resume_test.go`、`internal/cli/retry_indicator_test.go`、`internal/cli/review_test.go`、`internal/cli/rewind_test.go`、`internal/cli/run_metrics_test.go`、`internal/cli/select_test.go`、`internal/cli/skill_picker_test.go`、`internal/cli/skillslash_test.go`、`internal/cli/slash_test.go`、`internal/cli/statusline_test.go`、`internal/cli/theme_test.go`、`internal/cli/todopanel_test.go`、`internal/cli/toolcard_test.go`、`internal/cli/transcript_test.go`、`internal/cli/upgrade_test.go`、`internal/cli/view_helpers_test.go`、`internal/cli/width_test.go`
- **需同步检查：** `internal/cli`、`desktop/frontend/src/App.tsx`、`internal/cli/style.go`、`internal/cli/theme.go`、`internal/nilutil/nil.go`、`internal/control/controller.go`、`internal/config/paths.go`、`internal/automation/model_test.go`、`desktop/bound_array_contract_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`

#### 待修改项

- [ ] 对 `internal/cli` 下的生产文件按领域职责、平台、协议和入口进行分类，识别真正的零碎文件与合理边界。
- [ ] 只合并无独立职责、无平台/协议边界的碎片；不得为了文件数减少破坏清晰模块边界。
- [ ] 结合本包超长/复杂函数治理统一规划文件归属，避免多分支同时移动相同符号。
- [ ] 修复后重新扫描并显式确认 baseline key `package_files|internal/cli|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/cli` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/automation/model_test.go`、`desktop/bound_array_contract_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行全量 Code Guard，并确认 `package_files|internal/cli|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-123 · 圈复杂度 · `chatREPL`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-123 |
| Baseline key | `complexity\|internal/cli/cli.go\|chatREPL` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 52） |
| 文件 | `internal/cli/cli.go` |
| 代码行 | 652 |
| 函数/方法/包 | `chatREPL` |
| Baseline / 当前复测 / 阈值 | 37 / 37 / 10 |
| 超出 | +27，3.70x |
| 同一治理对象关联问题 | `FN-024` |

#### 明确的问题描述

`chatREPL` 的圈复杂度当前为 **37**，超过限制 **10**（+27，3.70x）。定义范围为 652-851 行；可见控制流热点：L663 `if err := fs.Parse(args); err != nil {`；L685 `case *cont:`；L726 `if err != nil {`；L778 `if m.ctrl != nil {`；L802 `if entry, ok := cfg.ResolveModel(name); ok {`；L846 `if runErr != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadCLIConfigForActor、LoadSession、defaultCLIProfilePathResolver、resolveCLISessionDirForActor）、状态与控制（setupWithActorAndExpert、setupQuietWithActorAndExpert、interactiveSetup、Controller::AdoptHistory）、持久化/I-O（defaultCLIProfilePathResolver）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function chatREPL（internal/cli/cli.go:652-851）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（21）：** `loadCLIConfigForActor`（`internal/cli/cli.go:248`，调用点 L670/L737/L765/L797，confidence=0.90，exact-match，4 条边）；`setupWithActorAndExpert`（`internal/cli/cli.go:209`，调用点 L705/L714，confidence=0.90，exact-match，2 条边）；`LoadSession`（`internal/agent/save.go:62`，调用点 L725，confidence=0.90，import）；`ListSessions`（`internal/agent/save.go:411`，调用点 L686，confidence=0.90，import）；`ContinueSessionPath`（`internal/agent/save.go:523`，调用点 L789，confidence=0.90，import）；`newChatTUI`（`internal/cli/chat_tui.go:452`，调用点 L764，confidence=0.90，exact-match）；`defaultCLIActorContext`（`internal/cli/cli.go:225`，调用点 L669，confidence=0.90，exact-match）；`defaultCLIProfilePathResolver`（`internal/cli/cli.go:240`，调用点 L711，confidence=0.90，exact-match）
- **待源码复核被调候选（7）：** `chatTUI::bottomRows`（`internal/cli/chat_tui.go:1465`，调用点 L809，confidence=0.70，instance-method，待源码复核）；`chatTUI::refreshEffortStatus`（`internal/cli/effort.go:128`，调用点 L806，confidence=0.70，instance-method，待源码复核）；`mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L663，confidence=0.70，instance-method，待源码复核）；`Config::UITheme`（`internal/config/config.go:127`，调用点 L672，confidence=0.70，instance-method，待源码复核）；`Config::UIThemeStyle`（`internal/config/config.go:140`，调用点 L672，confidence=0.70，instance-method，待源码复核）；`Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L802，confidence=0.70，instance-method，待源码复核）；`Controller::AdoptHistory`（`internal/control/sessionpath.go:31`，调用点 L790，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 41 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 8 个文件；`internal/cli`、`internal/agent`、`internal/config`、`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/cli.go`、`internal/agent/save.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `chatREPL` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L663、L706、L750、L797、L846 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `chatREPL` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-123、FN-024 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/cli.go|chatREPL` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/cli.go:652-851` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatREPL` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-123`、`FN-024` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 race：`go test -race ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/cli.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/cli.go|chatREPL` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-024 · 函数有效行数 · `chatREPL`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-024 |
| Baseline key | `func_lines\|internal/cli/cli.go\|chatREPL` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 52） |
| 文件 | `internal/cli/cli.go` |
| 代码行 | 652 |
| 函数/方法/包 | `chatREPL` |
| Baseline / 当前复测 / 阈值 | 148 / 148 / 80 |
| 超出 | +68，1.85x |
| 同一治理对象关联问题 | `CX-123` |

#### 明确的问题描述

`chatREPL` 的有效代码行 **148**，超过限制 **80**（+68）。CodeGraph 定义物理范围为 652-851 行，并记录 28 个直接下游候选；候选职责分布为 配置/解析（loadCLIConfigForActor、LoadSession、defaultCLIProfilePathResolver、resolveCLISessionDirForActor）、状态与控制（setupWithActorAndExpert、setupQuietWithActorAndExpert、interactiveSetup、Controller::AdoptHistory）、持久化/I-O（defaultCLIProfilePathResolver）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function chatREPL（internal/cli/cli.go:652-851）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（21）：** `loadCLIConfigForActor`（`internal/cli/cli.go:248`，调用点 L670/L737/L765/L797，confidence=0.90，exact-match，4 条边）；`setupWithActorAndExpert`（`internal/cli/cli.go:209`，调用点 L705/L714，confidence=0.90，exact-match，2 条边）；`LoadSession`（`internal/agent/save.go:62`，调用点 L725，confidence=0.90，import）；`ListSessions`（`internal/agent/save.go:411`，调用点 L686，confidence=0.90，import）；`ContinueSessionPath`（`internal/agent/save.go:523`，调用点 L789，confidence=0.90，import）；`newChatTUI`（`internal/cli/chat_tui.go:452`，调用点 L764，confidence=0.90，exact-match）；`defaultCLIActorContext`（`internal/cli/cli.go:225`，调用点 L669，confidence=0.90，exact-match）；`defaultCLIProfilePathResolver`（`internal/cli/cli.go:240`，调用点 L711，confidence=0.90，exact-match）
- **待源码复核被调候选（7）：** `chatTUI::bottomRows`（`internal/cli/chat_tui.go:1465`，调用点 L809，confidence=0.70，instance-method，待源码复核）；`chatTUI::refreshEffortStatus`（`internal/cli/effort.go:128`，调用点 L806，confidence=0.70，instance-method，待源码复核）；`mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L663，confidence=0.70，instance-method，待源码复核）；`Config::UITheme`（`internal/config/config.go:127`，调用点 L672，confidence=0.70，instance-method，待源码复核）；`Config::UIThemeStyle`（`internal/config/config.go:140`，调用点 L672，confidence=0.70，instance-method，待源码复核）；`Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L802，confidence=0.70，instance-method，待源码复核）；`Controller::AdoptHistory`（`internal/control/sessionpath.go:31`，调用点 L790，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 41 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 8 个文件；`internal/cli`、`internal/agent`、`internal/config`、`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/cli.go`、`internal/agent/save.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `chatREPL` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `chatREPL` 的 148 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L675 `Decide whether we're starting fresh or resuming. --resume opens an`、L734 `Surface a missing-key warning inside the TUI banner so the first message`、L787 `Keep the carried conversation in its existing file so the switch doesn't`、L833 `when executed while the TUI is alive.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-123、FN-024 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/cli.go|chatREPL` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/cli.go:652-851` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatREPL` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-123`、`FN-024` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 race：`go test -race ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/cli.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/cli.go|chatREPL` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-121 · 圈复杂度 · `runAgent`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-121 |
| Baseline key | `complexity\|internal/cli/cli.go\|runAgent` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 50） |
| 文件 | `internal/cli/cli.go` |
| 代码行 | 387 |
| 函数/方法/包 | `runAgent` |
| Baseline / 当前复测 / 阈值 | 26 / 26 / 10 |
| 超出 | +16，2.60x |
| 同一治理对象关联问题 | `FN-022` |

#### 明确的问题描述

`runAgent` 的圈复杂度当前为 **26**，超过限制 **10**（+16，2.60x）。定义范围为 387-502 行；可见控制流热点：L398 `if err := fs.Parse(args); err != nil {`；L419 `if *resume != "" {`；L437 `if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {`；L459 `if err != nil {`；L478 `if err != nil {`；L498 `if metricsErr != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（resolveCLISessionDirForActor、LoadSession、configureCLIThemeFromConfigForTTYOutput、loadCLIConfigForActor）、状态与控制（setupWithActorAndExpert、sendRunDoneNotification、TextSink::SetShowReasoning）、持久化/I-O（readStdin、writeMetricsIfRequested）、渲染/输出（newMarkdownRenderer）、网络/协议（writeMetricsIfRequested）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function runAgent（internal/cli/cli.go:387-502）
- **可信生产调用方（1）：** `Run`（`internal/cli/cli.go:51`，调用点 L91，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（18）：** `ListSessions`（`internal/agent/save.go:411`，调用点 L454/L472，confidence=0.90，import，2 条边）；`resolveCLISessionDirForActor`（`internal/cli/cli.go:259`，调用点 L421/L454，confidence=0.90，exact-match，2 条边）；`modelForResumePath`（`internal/cli/cli.go:310`，调用点 L452/L455，confidence=0.90，exact-match，2 条边）；`LoadSession`（`internal/agent/save.go:62`，调用点 L477，confidence=0.90，import）；`NewSessionPath`（`internal/agent/save.go:536`，调用点 L485，confidence=0.90，import）；`NewTextSink`（`internal/agent/textsink.go:41`，调用点 L442，confidence=0.90，import）；`configureCLIThemeFromConfigForTTYOutput`（`internal/cli/cli.go:181`，调用点 L406，confidence=0.90，exact-match）；`setupWithActorAndExpert`（`internal/cli/cli.go:209`，调用点 L458，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `TextSink::SetShowReasoning`（`internal/agent/textsink.go:48`，调用点 L443，confidence=0.80，instance-method，待源码复核）；`mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L398，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L91，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 36 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/cli`、`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/cli_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/cli.go`、`internal/agent/save.go`、`internal/cli/cli_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `runAgent` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L398、L422、L451、L478、L498 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `runAgent` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-121、FN-022 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/cli.go|runAgent` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/cli.go:387-502` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `runAgent` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-121`、`FN-022` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/cli_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/cli.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/cli.go|runAgent` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-022 · 函数有效行数 · `runAgent`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-022 |
| Baseline key | `func_lines\|internal/cli/cli.go\|runAgent` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 50） |
| 文件 | `internal/cli/cli.go` |
| 代码行 | 387 |
| 函数/方法/包 | `runAgent` |
| Baseline / 当前复测 / 阈值 | 101 / 101 / 80 |
| 超出 | +21，1.26x |
| 同一治理对象关联问题 | `CX-121` |

#### 明确的问题描述

`runAgent` 的有效代码行 **101**，超过限制 **80**（+21）。CodeGraph 定义物理范围为 387-502 行，并记录 20 个直接下游候选；候选职责分布为 配置/解析（resolveCLISessionDirForActor、LoadSession、configureCLIThemeFromConfigForTTYOutput、loadCLIConfigForActor）、状态与控制（setupWithActorAndExpert、sendRunDoneNotification、TextSink::SetShowReasoning）、持久化/I-O（readStdin、writeMetricsIfRequested）、渲染/输出（newMarkdownRenderer）、网络/协议（writeMetricsIfRequested）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function runAgent（internal/cli/cli.go:387-502）
- **可信生产调用方（1）：** `Run`（`internal/cli/cli.go:51`，调用点 L91，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（18）：** `ListSessions`（`internal/agent/save.go:411`，调用点 L454/L472，confidence=0.90，import，2 条边）；`resolveCLISessionDirForActor`（`internal/cli/cli.go:259`，调用点 L421/L454，confidence=0.90，exact-match，2 条边）；`modelForResumePath`（`internal/cli/cli.go:310`，调用点 L452/L455，confidence=0.90，exact-match，2 条边）；`LoadSession`（`internal/agent/save.go:62`，调用点 L477，confidence=0.90，import）；`NewSessionPath`（`internal/agent/save.go:536`，调用点 L485，confidence=0.90，import）；`NewTextSink`（`internal/agent/textsink.go:41`，调用点 L442，confidence=0.90，import）；`configureCLIThemeFromConfigForTTYOutput`（`internal/cli/cli.go:181`，调用点 L406，confidence=0.90，exact-match）；`setupWithActorAndExpert`（`internal/cli/cli.go:209`，调用点 L458，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `TextSink::SetShowReasoning`（`internal/agent/textsink.go:48`，调用点 L443，confidence=0.80，instance-method，待源码复核）；`mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L398，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L91，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 36 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/cli`、`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/cli_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/cli.go`、`internal/agent/save.go`、`internal/cli/cli_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `runAgent` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `runAgent` 的 101 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L431 `Live run: render the agent's event stream to stdout. Markdown post-stream`、L433 `keeps the raw stream.`、L466 `MCP/API callers that manage their own per-project session). Takes`、L468 `--continue: resume the most recent saved session.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-121、FN-022 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/cli.go|runAgent` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/cli.go:387-502` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `runAgent` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-121`、`FN-022` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/cli_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/cli.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/cli.go|runAgent` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FL-007 · 文件有效行数 · `internal/cli/cli.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-007 |
| Baseline key | `file_lines\|internal/cli/cli.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 49） |
| 文件 | `internal/cli/cli.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/cli/cli.go` |
| Baseline / 当前复测 / 阈值 | 1638 / 1638 / 800 |
| 超出 | +838，2.05x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/cli/cli.go` 当前有效代码行 **1638**，超过限制 **800**（+838）。CodeGraph 在文件中索引到的主要大符号包括 chatREPL@652-851、runServe@508-635、runAgent@387-502、Run@51-137、selectEnabledProviders@1120-1204、configReasoningLanguageCommand@1974-2038；文件同时承载的调用影响覆盖 24 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 88 个函数/方法节点。
- **可信生产调用方（4）：** `botDoctor`（`internal/cli/bot.go:164`，调用点 L172，confidence=0.90，exact-match）；`botWeixinLogin`（`internal/cli/bot.go:319`，调用点 L326，confidence=0.90，exact-match）；`loadBotCommandConfig`（`internal/cli/bot.go:352`，调用点 L353，confidence=0.90，exact-match）；`botCommandActor`（`internal/cli/bot.go:377`，调用点 L378，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（27）：** `dim`（`internal/cli/style.go:49`，调用点 L1123/L1180/L1009/L1513，confidence=0.90，exact-match，11 条边）；`green`（`internal/cli/style.go:50`，调用点 L1033/L1044/L1525/L1543，confidence=0.90，exact-match，10 条边）；`ListSessions`（`internal/agent/save.go:411`，调用点 L686/L454/L472/L1060，confidence=0.90，import，4 条边）；`selectMany`（`internal/cli/select.go:267`，调用点 L1138/L1188/L1531/L1637，confidence=0.90，exact-match，4 条边）；`selectOne`（`internal/cli/select.go:81`，调用点 L1085/L1104/L1443/L1549，confidence=0.90，exact-match，4 条边）；`LoadForEdit`（`internal/config/load.go:392`，调用点 L2021/L991/L1961/L1914，confidence=0.90，import，4 条边）；`DetectLanguage`（`internal/i18n/i18n.go:444`，调用点 L55/L72/L1001/L1097，confidence=0.90，import，4 条边）；`LoadSession`（`internal/agent/save.go:62`，调用点 L725/L477/L331，confidence=0.90，import，3 条边）
- **待源码复核被调候选（13）：** `mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L663/L519/L398/L1977，confidence=0.70，instance-method，6 条边，待源码复核）；`Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L802/L1718/L321，confidence=0.70，instance-method，3 条边，待源码复核）；`Config::SaveTo`（`internal/config/edit.go:809`，调用点 L2028/L1029/L962，confidence=0.70，instance-method，3 条边，待源码复核）；`Config::SaveToScope`（`internal/config/edit.go:820`，调用点 L2030/L1966/L1919，confidence=0.70，instance-method，3 条边，待源码复核）；`Config::UITheme`（`internal/config/config.go:127`，调用点 L672/L175，confidence=0.70，instance-method，2 条边，待源码复核）；`Config::UIThemeStyle`（`internal/config/config.go:140`，调用点 L672/L175，confidence=0.70，instance-method，2 条边，待源码复核）；`Config::ReasoningLanguage`（`internal/config/config.go:396`，调用点 L1991/L2036，confidence=0.70，instance-method，2 条边，待源码复核）；`Config::MemoryCompilerEnabled`（`internal/config/config.go:883`，调用点 L1948/L1970，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 422 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 24 个文件；`internal/cli`、`internal/agent`、`internal/config`、`internal/i18n`、`internal/boot`、`internal/serve`、`internal/userctx`、`internal/automation`、`internal/billing`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/cli_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/cli.go`、`internal/cli/bot.go`、`internal/cli/style.go`、`internal/cli/mathnode.go`、`internal/agent/save.go`、`internal/cli/cli_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `internal/cli/cli.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `chatREPL`、`runServe`、`runAgent`、`Run`、`selectEnabledProviders` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-120、CX-121、CX-122、CX-123、CX-124、CX-125、CX-126、CX-127、CX-128、FN-022、FN-023、FN-024。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/cli/cli.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/cli.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/cli/cli.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/cli_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 race：`go test -race ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/cli.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/cli/cli.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-122 · 圈复杂度 · `runServe`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-122 |
| Baseline key | `complexity\|internal/cli/cli.go\|runServe` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 49） |
| 文件 | `internal/cli/cli.go` |
| 代码行 | 508 |
| 函数/方法/包 | `runServe` |
| Baseline / 当前复测 / 阈值 | 24 / 24 / 10 |
| 超出 | +14，2.40x |
| 同一治理对象关联问题 | `FN-023` |

#### 明确的问题描述

`runServe` 的圈复杂度当前为 **24**，超过限制 **10**（+14，2.40x）。定义范围为 508-635 行；可见控制流热点：L519 `if err := fs.Parse(args); err != nil {`；L545 `if *auth != "" {`；L555 `if err != nil {`；L576 `if err != nil {`；L593 `if err != nil {`；L630 `if err := srv.RunGraceful(serveCtx, *addr); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadCLIConfigForActor、resolveCLISessionDirForActor、loadCLIResumableSession、NormalizeAuthMode）、状态与控制（setupWithActorAndExpert、Server::SetAutomationService、Server::RunGraceful）、权限/安全（NormalizeAuthMode、Server::AuthMode、Server::AuthToken）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function runServe（internal/cli/cli.go:508-635）
- **可信生产调用方（1）：** `Run`（`internal/cli/cli.go:51`，调用点 L95，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（12）：** `HashPassword`（`internal/serve/auth.go:236`，调用点 L529/L563，confidence=0.90，import，2 条边）；`setupWithActorAndExpert`（`internal/cli/cli.go:209`，调用点 L592，confidence=0.90，exact-match）；`defaultCLIActorContext`（`internal/cli/cli.go:225`，调用点 L540，confidence=0.90，exact-match）；`loadCLIConfigForActor`（`internal/cli/cli.go:248`，调用点 L541，confidence=0.90，exact-match）；`resolveCLISessionDirForActor`（`internal/cli/cli.go:259`，调用点 L580，confidence=0.90，exact-match）；`loadCLIResumableSession`（`internal/cli/cli.go:334`，调用点 L585，confidence=0.90，exact-match）；`newServeAutomation`（`internal/cli/cli.go:637`，调用点 L608，confidence=0.90，exact-match）；`serveModelDefault`（`internal/cli/model.go:105`，调用点 L591，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `Server::AuthMode`（`internal/serve/serve.go:98`，调用点 L614/L617，confidence=0.70，instance-method，2 条边，待源码复核）；`Balance::Display`（`internal/billing/balance.go:116`，调用点 L626，confidence=0.70，instance-method，待源码复核）；`mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L519，confidence=0.70，instance-method，待源码复核）；`Server::SetAutomationService`（`internal/serve/automation.go:12`，调用点 L612，confidence=0.70，instance-method，待源码复核）；`Server::RunGraceful`（`internal/serve/run.go:24`，调用点 L630，confidence=0.70，instance-method，待源码复核）；`Server::AuthToken`（`internal/serve/serve.go:90`，调用点 L616，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L95，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 39 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 9 个文件；`internal/cli`、`internal/serve`、`internal/billing`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/cli_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/cli.go`、`internal/serve/auth.go`、`internal/cli/cli_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `runServe` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L519、L548、L564、L593、L630 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、权限/安全 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `runServe` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-122、FN-023 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/cli.go|runServe` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/cli.go:508-635` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `runServe` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-122`、`FN-023` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/cli_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/cli.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/cli.go|runServe` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-023 · 函数有效行数 · `runServe`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-023 |
| Baseline key | `func_lines\|internal/cli/cli.go\|runServe` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 49） |
| 文件 | `internal/cli/cli.go` |
| 代码行 | 508 |
| 函数/方法/包 | `runServe` |
| Baseline / 当前复测 / 阈值 | 110 / 110 / 80 |
| 超出 | +30，1.38x |
| 同一治理对象关联问题 | `CX-122` |

#### 明确的问题描述

`runServe` 的有效代码行 **110**，超过限制 **80**（+30）。CodeGraph 定义物理范围为 508-635 行，并记录 18 个直接下游候选；候选职责分布为 配置/解析（loadCLIConfigForActor、resolveCLISessionDirForActor、loadCLIResumableSession、NormalizeAuthMode）、状态与控制（setupWithActorAndExpert、Server::SetAutomationService、Server::RunGraceful）、权限/安全（NormalizeAuthMode、Server::AuthMode、Server::AuthToken）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function runServe（internal/cli/cli.go:508-635）
- **可信生产调用方（1）：** `Run`（`internal/cli/cli.go:51`，调用点 L95，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（12）：** `HashPassword`（`internal/serve/auth.go:236`，调用点 L529/L563，confidence=0.90，import，2 条边）；`setupWithActorAndExpert`（`internal/cli/cli.go:209`，调用点 L592，confidence=0.90，exact-match）；`defaultCLIActorContext`（`internal/cli/cli.go:225`，调用点 L540，confidence=0.90，exact-match）；`loadCLIConfigForActor`（`internal/cli/cli.go:248`，调用点 L541，confidence=0.90，exact-match）；`resolveCLISessionDirForActor`（`internal/cli/cli.go:259`，调用点 L580，confidence=0.90，exact-match）；`loadCLIResumableSession`（`internal/cli/cli.go:334`，调用点 L585，confidence=0.90，exact-match）；`newServeAutomation`（`internal/cli/cli.go:637`，调用点 L608，confidence=0.90，exact-match）；`serveModelDefault`（`internal/cli/model.go:105`，调用点 L591，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `Server::AuthMode`（`internal/serve/serve.go:98`，调用点 L614/L617，confidence=0.70，instance-method，2 条边，待源码复核）；`Balance::Display`（`internal/billing/balance.go:116`，调用点 L626，confidence=0.70，instance-method，待源码复核）；`mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L519，confidence=0.70，instance-method，待源码复核）；`Server::SetAutomationService`（`internal/serve/automation.go:12`，调用点 L612，confidence=0.70，instance-method，待源码复核）；`Server::RunGraceful`（`internal/serve/run.go:24`，调用点 L630，confidence=0.70，instance-method，待源码复核）；`Server::AuthToken`（`internal/serve/serve.go:90`，调用点 L616，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L95，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 39 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 9 个文件；`internal/cli`、`internal/serve`、`internal/billing`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/cli_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/cli.go`、`internal/serve/auth.go`、`internal/cli/cli_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `runServe` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `runServe` 的 110 个有效行按 配置/解析、状态与控制、权限/安全 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L523 `--hash-password: generate a bcrypt hash and exit.`、L561 `Hash the password at startup so the config never stores plaintext.`、L599 `Auto-save target: reuse the resumed file, else a fresh one — same as chat.`、L629 `Use graceful shutdown so SIGINT/SIGTERM drain active connections.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-122、FN-023 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/cli.go|runServe` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/cli.go:508-635` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `runServe` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-122`、`FN-023` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/cli_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/cli.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/cli.go|runServe` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-120 · 圈复杂度 · `Run`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-120 |
| Baseline key | `complexity\|internal/cli/cli.go\|Run` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P1，治理对象得分 43） |
| 文件 | `internal/cli/cli.go` |
| 代码行 | 51 |
| 函数/方法/包 | `Run` |
| Baseline / 当前复测 / 阈值 | 27 / 27 / 10 |
| 超出 | +17，2.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Run` 的圈复杂度当前为 **27**，超过限制 **10**（+17，2.70x）。定义范围为 51-137 行；可见控制流热点：L57 `if len(args) > 0 {`；L71 `if cfg.Language != "" {`；L89 `switch cmd {`；L99 `case "config":`；L114 `case "doctor":`；L129 `case "help", "--help", "-h":`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（configureCLIThemeFromConfigNoProbe、configureCLIThemeFromConfigForTTYOutput、shouldMigrateLegacyConfigForCLI、migrateLegacyConfigForCLI）、状态与控制（runInteractiveSession、runAgent、runServe、setupConfig）、持久化/I-O（shouldMigrateLegacyConfigForCLI、migrateLegacyConfigForCLI）、渲染/输出（reviewCommand）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function Run（internal/cli/cli.go:51-137）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（21）：** `configureCLIThemeFromConfigNoProbe`（`internal/cli/cli.go:189`，调用点 L100/L106/L109/L112，confidence=0.90，exact-match，8 条边）；`runInteractiveSession`（`internal/cli/cli.go:44`，调用点 L77/L85/L93，confidence=0.90，exact-match，3 条边）；`configureCLIThemeFromConfigForTTYOutput`（`internal/cli/cli.go:181`，调用点 L80/L97，confidence=0.90，exact-match，2 条边）；`DetectLanguage`（`internal/i18n/i18n.go:444`，调用点 L55/L72，confidence=0.90，import，2 条边）；`acpCommand`（`internal/cli/acp.go:34`，调用点 L110，confidence=0.90，exact-match）；`botCommand`（`internal/cli/bot.go:21`，调用点 L122，confidence=0.90，exact-match）；`cliIsInteractive`（`internal/cli/cli.go:45`，调用点 L76，confidence=0.90，exact-match）；`isDefaultInteractiveFlag`（`internal/cli/cli.go:139`，调用点 L63，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `usage`（`internal/cli/cli.go:1861`，调用点 L81/L130/L134，confidence=0.70，exact-match，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 6 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 8 个文件；`internal/cli`、`internal/i18n`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/cli_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/cli.go`、`internal/i18n/i18n.go`、`internal/cli/cli_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `Run` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L57、L76、L94、L111、L129 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Run` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/cli.go|Run` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/cli.go:51-137` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Run` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/cli_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/cli.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/cli.go|Run` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-124 · 圈复杂度 · `selectEnabledProviders`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-124 |
| Baseline key | `complexity\|internal/cli/cli.go\|selectEnabledProviders` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/cli/cli.go` |
| 代码行 | 1120 |
| 函数/方法/包 | `selectEnabledProviders` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`selectEnabledProviders` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 1120-1204 行；可见控制流热点：L1122 `for _, s := range stale {`；L1144 `for _, fi := range famIdxs {`；L1146 `case customIdx:`；L1156 `if err != nil {`；L1185 `for i, m := range models {`；L1198 `for _, idx := range famMembers[familyKey] {`。CodeGraph 记录的下游职责候选主要涉及 网络/协议（fetchOrFallback、promptCustomProvider、promptAnthropicProvider）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function selectEnabledProviders（internal/cli/cli.go:1120-1204）
- **可信生产调用方（1）：** `interactiveSetup`（`internal/cli/cli.go:987`，调用点 L1013，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（11）：** `selectMany`（`internal/cli/select.go:267`，调用点 L1138/L1188，confidence=0.90，exact-match，2 条边）；`dim`（`internal/cli/style.go:49`，调用点 L1123/L1180，confidence=0.90，exact-match，2 条边）；`familyStaticModels`（`internal/cli/cli.go:1210`，调用点 L1171，confidence=0.90，exact-match）；`ensureProbeKey`（`internal/cli/cli.go:1229`，调用点 L1176，confidence=0.90，exact-match）；`fetchOrFallback`（`internal/cli/cli.go:1248`，调用点 L1178，confidence=0.90，exact-match）；`buildFamilyEntries`（`internal/cli/cli.go:1315`，调用点 L1201，confidence=0.90，exact-match）；`filterStaleCustomEntries`（`internal/cli/cli.go:1370`，调用点 L1121，confidence=0.90，exact-match）；`promptCustomProvider`（`internal/cli/cli.go:1442`，调用点 L1147，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `chatREPL`（`internal/cli/cli.go:652`，调用点 L711，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 26 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/cli.go`、`internal/cli/select.go`、`internal/cli/style.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `selectEnabledProviders` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1122、L1144、L1154、L1185、L1198 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `selectEnabledProviders` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/cli.go|selectEnabledProviders` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/cli.go:1120-1204` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `selectEnabledProviders` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/cli.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/cli.go|selectEnabledProviders` 下降或消失且无新增/恶化。
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
