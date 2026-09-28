# G15 · CLI/TUI 状态更新与交互渲染

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G15 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/cli` |
| 涉及文件 | `internal/cli/upgrade.go`<br>`internal/cli/effort.go`<br>`internal/cli/acp.go`<br>`internal/cli/chat_tui.go` |
| 主要治理目标 | 围绕 `internal/cli/chat_tui.go` 等治理对象降低复杂度/体量，统一处理 4 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | cli 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 20 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 15 组；等待 G02 的同文件变更合并 |
| 前置依赖 | G02（internal/cli/chat_tui.go） |
| 后续验证重点 | 协议、序列化或流式事件兼容；配置、平台或向后兼容；持久化、迁移或文件格式兼容；会话、状态或恢复语义；并发、取消或事件顺序；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **52/100**，对象 `upgradeCommand` 的分项为严重度 7.2、集中度 8.6、影响面 19.8、可执行性 16.0；最高超限 1.90x，文件内 2 条问题，CodeGraph 记录 1 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数，必须合并治理。

### 治理边界与组间关系

本组涉及 4 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G02。后续同文件组：G36、G39，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-170` | `complexity\|internal/cli/upgrade.go\|upgradeCommand` | 圈复杂度 | `internal/cli/upgrade.go` | `upgradeCommand` | 50 | 19/10 | +9（1.90x） | P2 | 1 可信 + 0 待核 caller / 7 可信 + 5 待核 callee / 27 unresolved | 待修改 |
| `FN-029` | `func_lines\|internal/cli/upgrade.go\|upgradeCommand` | 函数有效行数 | `internal/cli/upgrade.go` | `upgradeCommand` | 50 | 92/80 | +12（1.15x） | P2 | 1 可信 + 0 待核 caller / 7 可信 + 5 待核 callee / 27 unresolved | 待修改 |
| `CX-134` | `complexity\|internal/cli/effort.go\|*chatTUI.runEffortCommand` | 圈复杂度 | `internal/cli/effort.go` | `*chatTUI.runEffortCommand` | 12 | 20/10 | +10（2.00x） | P2 | 0 可信 + 1 待核 caller / 3 可信 + 6 待核 callee / 0 unresolved | 待修改 |
| `FN-025` | `func_lines\|internal/cli/effort.go\|*chatTUI.runEffortCommand` | 函数有效行数 | `internal/cli/effort.go` | `*chatTUI.runEffortCommand` | 12 | 91/80 | +11（1.14x） | P2 | 0 可信 + 1 待核 caller / 3 可信 + 6 待核 callee / 0 unresolved | 待修改 |
| `CX-102` | `complexity\|internal/cli/acp.go\|*acpFactory.SessionConfigState` | 圈复杂度 | `internal/cli/acp.go` | `*acpFactory.SessionConfigState` | 96 | 17/10 | +7（1.70x） | P2 | 0 可信 + 0 待核 caller / 8 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `FN-015` | `func_lines\|internal/cli/acp.go\|*acpFactory.SessionConfigState` | 函数有效行数 | `internal/cli/acp.go` | `*acpFactory.SessionConfigState` | 96 | 97/80 | +17（1.21x） | P2 | 0 可信 + 0 待核 caller / 8 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-109` | `complexity\|internal/cli/chat_tui.go\|*chatTUI.collapseToolOutput` | 圈复杂度 | `internal/cli/chat_tui.go` | `*chatTUI.collapseToolOutput` | 1765 | 16/10 | +6（1.60x） | P2 | 0 可信 + 3 待核 caller / 3 可信 + 2 待核 callee / 11 unresolved | 待修改 |
| `ND-008` | `nesting_depth\|internal/cli/chat_tui.go\|*chatTUI.collapseToolOutput` | 嵌套深度 | `internal/cli/chat_tui.go` | `*chatTUI.collapseToolOutput` | 1765 | 5/4 | +1（1.25x） | P2 | 0 可信 + 3 待核 caller / 3 可信 + 2 待核 callee / 11 unresolved | 待修改 |
| `CX-114` | `complexity\|internal/cli/chat_tui.go\|chatTUI.computeStatusLineCount` | 圈复杂度 | `internal/cli/chat_tui.go` | `chatTUI.computeStatusLineCount` | 2717 | 25/10 | +15（2.50x） | P2 | 0 可信 + 1 待核 caller / 2 可信 + 10 待核 callee / 11 unresolved | 待修改 |
| `CX-115` | `complexity\|internal/cli/chat_tui.go\|chatTUI.modeTagText` | 圈复杂度 | `internal/cli/chat_tui.go` | `chatTUI.modeTagText` | 2872 | 25/10 | +15（2.50x） | P3 | 0 可信 + 2 待核 caller / 0 可信 + 1 待核 callee / 1 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-170 · 圈复杂度 · `upgradeCommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-170 |
| Baseline key | `complexity\|internal/cli/upgrade.go\|upgradeCommand` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 52） |
| 文件 | `internal/cli/upgrade.go` |
| 代码行 | 50 |
| 函数/方法/包 | `upgradeCommand` |
| Baseline / 当前复测 / 阈值 | 19 / 19 / 10 |
| 超出 | +9，1.90x |
| 同一治理对象关联问题 | `FN-029` |

#### 明确的问题描述

`upgradeCommand` 的圈复杂度当前为 **19**，超过限制 **10**（+9，1.90x）。定义范围为 50-165 行；可见控制流热点：L54 `if err := fs.Parse(args); err != nil {`；L79 `if err != nil {`；L94 `if *force {`；L112 `if strings.HasPrefix(rel.Assets[i].Name, base) {`；L140 `if err := verifyChecksum(archiveData, asset.Name, checksumData); err != nil {`；L158 `if err := replaceBinary(binary); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Load、mathParser::Parse、normalizeVersion、Config::NetworkProxySpec）、网络/协议（fetchLatestRelease、NewHTTPClient、fetchBytes）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function upgradeCommand（internal/cli/upgrade.go:50-165）
- **可信生产调用方（1）：** `Run`（`internal/cli/cli.go:51`，调用点 L125，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（7）：** `upgradeSuccessMessage`（`internal/cli/upgrade.go:167`，调用点 L163，confidence=0.90，exact-match）；`fetchLatestRelease`（`internal/cli/upgrade.go:209`，调用点 L78，confidence=0.90，exact-match）；`verifyChecksum`（`internal/cli/upgrade.go:252`，调用点 L140，confidence=0.90，exact-match）；`replaceBinary`（`internal/cli/upgrade.go:333`，调用点 L158，confidence=0.90，exact-match）；`humanSize`（`internal/cli/upgrade.go:408`，调用点 L126，confidence=0.90，exact-match）；`Load`（`internal/config/load.go:21`，调用点 L66，confidence=0.90，import）；`NewHTTPClient`（`internal/netclient/netclient.go:83`，调用点 L68，confidence=0.90，import）
- **待源码复核被调候选（5）：** `fetchBytes`（`internal/cli/upgrade.go:238`，调用点 L127/L135，confidence=0.70，exact-match，2 条边，待源码复核）；`mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L54，confidence=0.70，instance-method，待源码复核）；`normalizeVersion`（`internal/cli/upgrade.go:172`，调用点 L59，confidence=0.70，exact-match，待源码复核）；`extractBinary`（`internal/cli/upgrade.go:273`，调用点 L150，confidence=0.70，exact-match，待源码复核）；`Config::NetworkProxySpec`（`internal/config/config.go:610`，调用点 L67，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L125，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 27 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/cli`、`internal/config`、`internal/netclient`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/upgrade.go`、`internal/cli/cli.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `upgradeCommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L54、L86、L111、L136、L158 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `upgradeCommand` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-170、FN-029 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/upgrade.go|upgradeCommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/upgrade.go:50-165` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `upgradeCommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G39 处理。
- 重点保护：协议、序列化或流式事件兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-170`、`FN-029` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/upgrade.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/upgrade.go|upgradeCommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-029 · 函数有效行数 · `upgradeCommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-029 |
| Baseline key | `func_lines\|internal/cli/upgrade.go\|upgradeCommand` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 52） |
| 文件 | `internal/cli/upgrade.go` |
| 代码行 | 50 |
| 函数/方法/包 | `upgradeCommand` |
| Baseline / 当前复测 / 阈值 | 92 / 92 / 80 |
| 超出 | +12，1.15x |
| 同一治理对象关联问题 | `CX-170` |

#### 明确的问题描述

`upgradeCommand` 的有效代码行 **92**，超过限制 **80**（+12）。CodeGraph 定义物理范围为 50-165 行，并记录 12 个直接下游候选；候选职责分布为 配置/解析（Load、mathParser::Parse、normalizeVersion、Config::NetworkProxySpec）、网络/协议（fetchLatestRelease、NewHTTPClient、fetchBytes）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function upgradeCommand（internal/cli/upgrade.go:50-165）
- **可信生产调用方（1）：** `Run`（`internal/cli/cli.go:51`，调用点 L125，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（7）：** `upgradeSuccessMessage`（`internal/cli/upgrade.go:167`，调用点 L163，confidence=0.90，exact-match）；`fetchLatestRelease`（`internal/cli/upgrade.go:209`，调用点 L78，confidence=0.90，exact-match）；`verifyChecksum`（`internal/cli/upgrade.go:252`，调用点 L140，confidence=0.90，exact-match）；`replaceBinary`（`internal/cli/upgrade.go:333`，调用点 L158，confidence=0.90，exact-match）；`humanSize`（`internal/cli/upgrade.go:408`，调用点 L126，confidence=0.90，exact-match）；`Load`（`internal/config/load.go:21`，调用点 L66，confidence=0.90，import）；`NewHTTPClient`（`internal/netclient/netclient.go:83`，调用点 L68，confidence=0.90，import）
- **待源码复核被调候选（5）：** `fetchBytes`（`internal/cli/upgrade.go:238`，调用点 L127/L135，confidence=0.70，exact-match，2 条边，待源码复核）；`mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L54，confidence=0.70，instance-method，待源码复核）；`normalizeVersion`（`internal/cli/upgrade.go:172`，调用点 L59，confidence=0.70，exact-match，待源码复核）；`extractBinary`（`internal/cli/upgrade.go:273`，调用点 L150，confidence=0.70，exact-match，待源码复核）；`Config::NetworkProxySpec`（`internal/config/config.go:610`，调用点 L67，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L125，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 27 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/cli`、`internal/config`、`internal/netclient`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/upgrade.go`、`internal/cli/cli.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `upgradeCommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `upgradeCommand` 的 92 个有效行按 配置/解析、网络/协议 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L58 `1. Normalize running version.`、L84 `4. Compare versions.`、L125 `7. Download archive.`、L156 `10. Replace the running binary.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-170、FN-029 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/upgrade.go|upgradeCommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/upgrade.go:50-165` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `upgradeCommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G39 处理。
- 重点保护：协议、序列化或流式事件兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-170`、`FN-029` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/upgrade.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/upgrade.go|upgradeCommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-134 · 圈复杂度 · `*chatTUI.runEffortCommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-134 |
| Baseline key | `complexity\|internal/cli/effort.go\|*chatTUI.runEffortCommand` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 51） |
| 文件 | `internal/cli/effort.go` |
| 代码行 | 12 |
| 函数/方法/包 | `*chatTUI.runEffortCommand` |
| Baseline / 当前复测 / 阈值 | 20 / 20 / 10 |
| 超出 | +10，2.00x |
| 同一治理对象关联问题 | `FN-025` |

#### 明确的问题描述

`*chatTUI.runEffortCommand` 的圈复杂度当前为 **20**，超过限制 **10**（+10，2.00x）。定义范围为 12-107 行；可见控制流热点：L14 `if err != nil {`；L31 `if len(args) > 2 {`；L44 `if m.ctrl.Running() {`；L61 `if entry.Kind == "anthropic" && effort != "" && entry.Thinking == "" {`；L71 `if err := edit.SaveToScope(path, config.RenderScopeUser); err != nil {`；L92 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（NormalizeEffort、chatTUI::currentConfigProvider、Config::UpsertProvider、Config::SetProviderEffort）、状态与控制（Config::SetProviderEffort、Config::SetProviderThinking）、持久化/I-O（Config::SaveToScope、ProfilePathResolver::UserConfigPath）、网络/协议（chatTUI::currentConfigProvider、Config::UpsertProvider、Config::SetProviderEffort、Config::SetProviderThinking）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::runEffortCommand（internal/cli/effort.go:12-107）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::runSlashCommand`（`internal/cli/chat_tui.go:3226`，调用点 L3270，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `EffortCapabilityForEntry`（`internal/config/effort.go:39`，调用点 L18，confidence=0.90，import）；`NormalizeEffort`（`internal/config/effort.go:86`，调用点 L35，confidence=0.90，import）；`EffortDisplay`（`internal/config/effort.go:182`，调用点 L26，confidence=0.90，import）
- **待源码复核被调候选（6）：** `chatTUI::currentConfigProvider`（`internal/cli/effort.go:109`，调用点 L13，confidence=0.70，instance-method，待源码复核）；`Config::UpsertProvider`（`internal/config/edit.go:120`，调用点 L56，confidence=0.70，instance-method，待源码复核）；`Config::SetProviderEffort`（`internal/config/edit.go:136`，调用点 L67，confidence=0.70，instance-method，待源码复核）；`Config::SetProviderThinking`（`internal/config/edit.go:349`，调用点 L62，confidence=0.70，instance-method，待源码复核）；`Config::SaveToScope`（`internal/config/edit.go:820`，调用点 L71，confidence=0.70，instance-method，待源码复核）；`ProfilePathResolver::UserConfigPath`（`internal/config/paths.go:176`，调用点 L50，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1173，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/cli`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/effort.go`、`internal/cli/chat_tui.go`、`internal/config/effort.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.runEffortCommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L14、L36、L55、L67、L92 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.runEffortCommand` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-134、FN-025 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/effort.go|*chatTUI.runEffortCommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/effort.go:12-107` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.runEffortCommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G39 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-134`、`FN-025` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/effort.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/effort.go|*chatTUI.runEffortCommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-025 · 函数有效行数 · `*chatTUI.runEffortCommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-025 |
| Baseline key | `func_lines\|internal/cli/effort.go\|*chatTUI.runEffortCommand` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 51） |
| 文件 | `internal/cli/effort.go` |
| 代码行 | 12 |
| 函数/方法/包 | `*chatTUI.runEffortCommand` |
| Baseline / 当前复测 / 阈值 | 91 / 91 / 80 |
| 超出 | +11，1.14x |
| 同一治理对象关联问题 | `CX-134` |

#### 明确的问题描述

`*chatTUI.runEffortCommand` 的有效代码行 **91**，超过限制 **80**（+11）。CodeGraph 定义物理范围为 12-107 行，并记录 9 个直接下游候选；候选职责分布为 配置/解析（NormalizeEffort、chatTUI::currentConfigProvider、Config::UpsertProvider、Config::SetProviderEffort）、状态与控制（Config::SetProviderEffort、Config::SetProviderThinking）、持久化/I-O（Config::SaveToScope、ProfilePathResolver::UserConfigPath）、网络/协议（chatTUI::currentConfigProvider、Config::UpsertProvider、Config::SetProviderEffort、Config::SetProviderThinking）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::runEffortCommand（internal/cli/effort.go:12-107）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::runSlashCommand`（`internal/cli/chat_tui.go:3226`，调用点 L3270，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `EffortCapabilityForEntry`（`internal/config/effort.go:39`，调用点 L18，confidence=0.90，import）；`NormalizeEffort`（`internal/config/effort.go:86`，调用点 L35，confidence=0.90，import）；`EffortDisplay`（`internal/config/effort.go:182`，调用点 L26，confidence=0.90，import）
- **待源码复核被调候选（6）：** `chatTUI::currentConfigProvider`（`internal/cli/effort.go:109`，调用点 L13，confidence=0.70，instance-method，待源码复核）；`Config::UpsertProvider`（`internal/config/edit.go:120`，调用点 L56，confidence=0.70，instance-method，待源码复核）；`Config::SetProviderEffort`（`internal/config/edit.go:136`，调用点 L67，confidence=0.70，instance-method，待源码复核）；`Config::SetProviderThinking`（`internal/config/edit.go:349`，调用点 L62，confidence=0.70，instance-method，待源码复核）；`Config::SaveToScope`（`internal/config/edit.go:820`，调用点 L71，confidence=0.70，instance-method，待源码复核）；`ProfilePathResolver::UserConfigPath`（`internal/config/paths.go:176`，调用点 L50，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1173，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/cli`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/effort.go`、`internal/cli/chat_tui.go`、`internal/config/effort.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.runEffortCommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*chatTUI.runEffortCommand` 的 91 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-134、FN-025 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/effort.go|*chatTUI.runEffortCommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/effort.go:12-107` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.runEffortCommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G39 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-134`、`FN-025` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/effort.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/effort.go|*chatTUI.runEffortCommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-102 · 圈复杂度 · `*acpFactory.SessionConfigState`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-102 |
| Baseline key | `complexity\|internal/cli/acp.go\|*acpFactory.SessionConfigState` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 49） |
| 文件 | `internal/cli/acp.go` |
| 代码行 | 96 |
| 函数/方法/包 | `*acpFactory.SessionConfigState` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | `FN-015` |

#### 明确的问题描述

`*acpFactory.SessionConfigState` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 96-198 行；可见控制流热点：L98 `if root == "" {`；L107 `if actor.ProfileID == userctx.DefaultProfileID {`；L117 `if strings.TrimSpace(ref) == "" {`；L129 `if !hasModelOption(modelOptions, currentModel) {`；L146 `if strings.TrimSpace(*effortOverride) == "" {`；L171 `if !containsString(cap.Levels, currentEffort) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（NormalizeEffort、MigrateMCPToUserConfigOnUpgrade）、持久化/I-O（MigrateLegacyIfNeededForRoot、MigrateMCPToUserConfigOnUpgrade）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method acpFactory::SessionConfigState（internal/cli/acp.go:96-198）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `acpModelOptions`（`internal/cli/acp.go:230`，调用点 L128，confidence=0.90，exact-match）；`hasModelOption`（`internal/cli/acp.go:258`，调用点 L129，confidence=0.90，exact-match）；`acpEffortOptions`（`internal/cli/acp.go:267`，调用点 L182，confidence=0.90，exact-match）；`EffortCapabilityForEntry`（`internal/config/effort.go:39`，调用点 L169，confidence=0.90，import）；`NormalizeEffort`（`internal/config/effort.go:86`，调用点 L149，confidence=0.90，import）；`EffortDisplay`（`internal/config/effort.go:182`，调用点 L170，confidence=0.90，import）；`MigrateLegacyIfNeededForRoot`（`internal/config/migrate.go:91`，调用点 L108，confidence=0.90，import）；`MigrateMCPToUserConfigOnUpgrade`（`internal/config/migrate.go:101`，调用点 L109，confidence=0.90，import）
- **待源码复核被调候选（3）：** `acpFactory::actorForRoot`（`internal/cli/acp.go:200`，调用点 L106，confidence=0.70，instance-method，待源码复核）；`firstNonEmpty`（`internal/cli/acp.go:285`，调用点 L116，confidence=0.70，exact-match，待源码复核）；`cloneStringPtr`（`internal/cli/acp.go:294`，调用点 L143，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/acp.go`、`internal/config/effort.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*acpFactory.SessionConfigState` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L98、L107、L124、L146、L171 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*acpFactory.SessionConfigState` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-102、FN-015 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/acp.go|*acpFactory.SessionConfigState` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/acp.go:96-198` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*acpFactory.SessionConfigState` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G39 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-102`、`FN-015` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/acp.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/acp.go|*acpFactory.SessionConfigState` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-015 · 函数有效行数 · `*acpFactory.SessionConfigState`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-015 |
| Baseline key | `func_lines\|internal/cli/acp.go\|*acpFactory.SessionConfigState` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 49） |
| 文件 | `internal/cli/acp.go` |
| 代码行 | 96 |
| 函数/方法/包 | `*acpFactory.SessionConfigState` |
| Baseline / 当前复测 / 阈值 | 97 / 97 / 80 |
| 超出 | +17，1.21x |
| 同一治理对象关联问题 | `CX-102` |

#### 明确的问题描述

`*acpFactory.SessionConfigState` 的有效代码行 **97**，超过限制 **80**（+17）。CodeGraph 定义物理范围为 96-198 行，并记录 11 个直接下游候选；候选职责分布为 配置/解析（NormalizeEffort、MigrateMCPToUserConfigOnUpgrade）、持久化/I-O（MigrateLegacyIfNeededForRoot、MigrateMCPToUserConfigOnUpgrade）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method acpFactory::SessionConfigState（internal/cli/acp.go:96-198）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `acpModelOptions`（`internal/cli/acp.go:230`，调用点 L128，confidence=0.90，exact-match）；`hasModelOption`（`internal/cli/acp.go:258`，调用点 L129，confidence=0.90，exact-match）；`acpEffortOptions`（`internal/cli/acp.go:267`，调用点 L182，confidence=0.90，exact-match）；`EffortCapabilityForEntry`（`internal/config/effort.go:39`，调用点 L169，confidence=0.90，import）；`NormalizeEffort`（`internal/config/effort.go:86`，调用点 L149，confidence=0.90，import）；`EffortDisplay`（`internal/config/effort.go:182`，调用点 L170，confidence=0.90，import）；`MigrateLegacyIfNeededForRoot`（`internal/config/migrate.go:91`，调用点 L108，confidence=0.90，import）；`MigrateMCPToUserConfigOnUpgrade`（`internal/config/migrate.go:101`，调用点 L109，confidence=0.90，import）
- **待源码复核被调候选（3）：** `acpFactory::actorForRoot`（`internal/cli/acp.go:200`，调用点 L106，confidence=0.70，instance-method，待源码复核）；`firstNonEmpty`（`internal/cli/acp.go:285`，调用点 L116，confidence=0.70，exact-match，待源码复核）；`cloneStringPtr`（`internal/cli/acp.go:294`，调用点 L143，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/acp.go`、`internal/config/effort.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*acpFactory.SessionConfigState` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*acpFactory.SessionConfigState` 的 97 个有效行按 配置/解析、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-102、FN-015 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/acp.go|*acpFactory.SessionConfigState` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/acp.go:96-198` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*acpFactory.SessionConfigState` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G39 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-102`、`FN-015` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/acp.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/acp.go|*acpFactory.SessionConfigState` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-109 · 圈复杂度 · `*chatTUI.collapseToolOutput`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-109 |
| Baseline key | `complexity\|internal/cli/chat_tui.go\|*chatTUI.collapseToolOutput` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 48） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 1765 |
| 函数/方法/包 | `*chatTUI.collapseToolOutput` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | `ND-008` |

#### 明确的问题描述

`*chatTUI.collapseToolOutput` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 1765-1819 行；可见控制流热点：L1766 `if m.nativeScrollback {`；L1771 `if m.toolPartial != "" {`；L1775 `if full, ok := m.shellOutputs[id]; ok {`；L1778 `if total > shellPreviewLines {`；L1787 `for i, ln := range lines {`；L1808 `if idx, ok := m.shellTranscriptIdx[id]; ok && idx >= 0 && idx < len(m.transcript) {`。CodeGraph 记录的下游职责候选主要涉及 任务/并发（connectorBlock）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::collapseToolOutput（internal/cli/chat_tui.go:1765-1819）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（3）：** `chatTUI::streamToolOutput`（`internal/cli/chat_tui.go:1672`，调用点 L1704，confidence=0.70，instance-method，待源码复核）；`chatTUI::ingestEvent`（`internal/cli/chat_tui.go:3004`，调用点 L3097，confidence=0.70，instance-method，待源码复核）；`chatTUI::finalizeStreamed`（`internal/cli/chat_tui.go:3210`，调用点 L3211，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `dim`（`internal/cli/style.go:49`，调用点 L1781/L1783/L1788/L1794，confidence=0.90，exact-match，4 条边）；`connectorBlock`（`internal/cli/toolcard.go:19`，调用点 L1784/L1790/L1794，confidence=0.90，exact-match，3 条边）；`clampPlain`（`internal/cli/diffview.go:205`，调用点 L1781/L1788，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（2）：** `chatTUI::commitLine`（`internal/cli/chat_tui.go:1446`，调用点 L1784/L1790/L1794，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::collapseShellSlot`（`internal/cli/chat_tui.go:1826`，调用点 L1809/L1813，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1200，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 11 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/style.go`、`internal/cli/toolcard.go`、`internal/cli/diffview.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.collapseToolOutput` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1766、L1771、L1778、L1787、L1808 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.collapseToolOutput` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-109、ND-008 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui.go|*chatTUI.collapseToolOutput` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:1765-1819` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.collapseToolOutput` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G39 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-109`、`ND-008` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui.go|*chatTUI.collapseToolOutput` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-008 · 嵌套深度 · `*chatTUI.collapseToolOutput`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-008 |
| Baseline key | `nesting_depth\|internal/cli/chat_tui.go\|*chatTUI.collapseToolOutput` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 48） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 1765 |
| 函数/方法/包 | `*chatTUI.collapseToolOutput` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-109` |

#### 明确的问题描述

`*chatTUI.collapseToolOutput` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L1766 `if m.nativeScrollback {`；L1771 `if m.toolPartial != "" {`；L1775 `if full, ok := m.shellOutputs[id]; ok {`；L1778 `if total > shellPreviewLines {`；L1787 `for i, ln := range lines {`；L1808 `if idx, ok := m.shellTranscriptIdx[id]; ok && idx >= 0 && idx < len(m.transcript) {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::collapseToolOutput（internal/cli/chat_tui.go:1765-1819）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（3）：** `chatTUI::streamToolOutput`（`internal/cli/chat_tui.go:1672`，调用点 L1704，confidence=0.70，instance-method，待源码复核）；`chatTUI::ingestEvent`（`internal/cli/chat_tui.go:3004`，调用点 L3097，confidence=0.70，instance-method，待源码复核）；`chatTUI::finalizeStreamed`（`internal/cli/chat_tui.go:3210`，调用点 L3211，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `dim`（`internal/cli/style.go:49`，调用点 L1781/L1783/L1788/L1794，confidence=0.90，exact-match，4 条边）；`connectorBlock`（`internal/cli/toolcard.go:19`，调用点 L1784/L1790/L1794，confidence=0.90，exact-match，3 条边）；`clampPlain`（`internal/cli/diffview.go:205`，调用点 L1781/L1788，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（2）：** `chatTUI::commitLine`（`internal/cli/chat_tui.go:1446`，调用点 L1784/L1790/L1794，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::collapseShellSlot`（`internal/cli/chat_tui.go:1826`，调用点 L1809/L1813，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1200，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 11 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/style.go`、`internal/cli/toolcard.go`、`internal/cli/diffview.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.collapseToolOutput` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 从 L1766、L1771、L1778、L1787、L1808 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-109、ND-008 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/cli/chat_tui.go|*chatTUI.collapseToolOutput` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:1765-1819` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.collapseToolOutput` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G39 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-109`、`ND-008` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/cli/chat_tui.go|*chatTUI.collapseToolOutput` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-114 · 圈复杂度 · `chatTUI.computeStatusLineCount`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-114 |
| Baseline key | `complexity\|internal/cli/chat_tui.go\|chatTUI.computeStatusLineCount` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 44） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 2717 |
| 函数/方法/包 | `chatTUI.computeStatusLineCount` |
| Baseline / 当前复测 / 阈值 | 25 / 25 / 10 |
| 超出 | +15，2.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.computeStatusLineCount` 的圈复杂度当前为 **25**，超过限制 **10**（+15，2.50x）。定义范围为 2717-2799 行；可见控制流热点：L2718 `if m.ctrl == nil {`；L2735 `case m.mcpImport != nil:`；L2745 `case m.pendingApproval != nil && m.pendingApproval.Tool == planApprovalTool:`；L2753 `case m.ctrl.AutoApproveTools():`；L2773 `if ct := m.contextTag(); ct != "" {`；L2792 `if m.state == tuiRunning {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（chatTUI::runningWorkingLine）、网络/协议（chatTUI::cancelRequested）、任务/并发（chatTUI::jobsTag）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::computeStatusLineCount（internal/cli/chat_tui.go:2717-2799）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::Update`（`internal/cli/chat_tui.go:670`，调用点 L684，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `wrapStatusLine`（`internal/cli/chat_tui.go:2705`，调用点 L2794/L2796/L2797，confidence=0.90，exact-match，3 条边）；`xlsxCell::Value`（`internal/automation/business.go:1089`，调用点 L2721，confidence=0.90，exact-match）
- **待源码复核被调候选（10）：** `chatTUI::cycleHint`（`internal/cli/chat_tui.go:2847`，调用点 L2754/L2756，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::cancelRequested`（`internal/cli/chat_tui.go:2170`，调用点 L2722，confidence=0.70，instance-method，待源码复核）；`chatTUI::runningWorkingLine`（`internal/cli/chat_tui.go:2177`，调用点 L2788，confidence=0.70，instance-method，待源码复核）；`chatTUI::contextTag`（`internal/cli/chat_tui.go:2442`，调用点 L2773，confidence=0.70，instance-method，待源码复核）；`chatTUI::cacheTag`（`internal/cli/chat_tui.go:2490`，调用点 L2770，confidence=0.70，instance-method，待源码复核）；`chatTUI::jobsTag`（`internal/cli/chat_tui.go:2517`，调用点 L2776，confidence=0.70，instance-method，待源码复核）；`chatTUI::modelTag`（`internal/cli/chat_tui.go:2525`，调用点 L2767，confidence=0.70，instance-method，待源码复核）；`chatTUI::effortTag`（`internal/cli/chat_tui.go:2532`，调用点 L2758，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L882，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 11 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/automation/business.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.computeStatusLineCount` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L2718、L2739、L2749、L2770、L2792 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、网络/协议、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `chatTUI.computeStatusLineCount` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui.go|chatTUI.computeStatusLineCount` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:2717-2799` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.computeStatusLineCount` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G39 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 race：`go test -race ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui.go|chatTUI.computeStatusLineCount` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-115 · 圈复杂度 · `chatTUI.modeTagText`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-115 |
| Baseline key | `complexity\|internal/cli/chat_tui.go\|chatTUI.modeTagText` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P1，治理对象得分 41） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 2872 |
| 函数/方法/包 | `chatTUI.modeTagText` |
| Baseline / 当前复测 / 阈值 | 25 / 25 / 10 |
| 超出 | +15，2.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.modeTagText` 的圈复杂度当前为 **25**，超过限制 **10**（+15，2.50x）。定义范围为 2872-2915 行；可见控制流热点：L2875 `if m.desktopShortcutLayout() {`；L2879 `case goalMode && toolApprovalMode == control.ToolApprovalYolo:`；L2887 `case goalMode:`；L2896 `case m.planMode && toolApprovalMode == control.ToolApprovalYolo:`；L2904 `case toolApprovalMode == control.ToolApprovalYolo:`；L2910 `case goalMode:`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::modeTagText（internal/cli/chat_tui.go:2872-2915）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `chatTUI::View`（`internal/cli/chat_tui.go:2210`，调用点 L2251，confidence=0.70，instance-method，待源码复核）；`chatTUI::computeStatusLineCount`（`internal/cli/chat_tui.go:2717`，调用点 L2727，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `chatTUI::desktopShortcutLayout`（`internal/cli/chat_tui.go:2843`，调用点 L2875，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::View`（`internal/cli/chat_tui.go:2210`，调用点 L2251，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 1 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.modeTagText` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L2875、L2881、L2895、L2902、L2910 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `chatTUI.modeTagText` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui.go|chatTUI.modeTagText` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:2872-2915` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.modeTagText` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G39 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui.go|chatTUI.modeTagText` 下降或消失且无新增/恶化。
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
