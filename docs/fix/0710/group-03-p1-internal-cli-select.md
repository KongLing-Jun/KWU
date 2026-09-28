# G03 · internal/cli · select 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G03 |
| 问题数量 | 10 |
| 整体优先级 | P1 |
| 涉及模块 | `internal/cli` |
| 涉及文件 | `internal/cli/select.go`<br>`internal/cli/mcp_manager.go`<br>`internal/cli/bot.go` |
| 主要治理目标 | 围绕 `internal/cli/select.go` 等治理对象降低复杂度/体量，统一处理 4 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | cli 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614`、`origin/expert-202607081622`；单文件最多集中 5 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 3 组；先于 G22 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 协议、序列化或流式事件兼容；会话、状态或恢复语义；返回值、错误语义和现有调用顺序；配置、平台或向后兼容；持久化、迁移或文件格式兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **70/100**，对象 `selectMany` 的分项为严重度 20.0、集中度 15.9、影响面 22.2、可执行性 12.0；最高超限 7.30x，文件内 5 条问题，CodeGraph 记录 3 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数、嵌套深度，必须合并治理。

### 治理边界与组间关系

本组涉及 3 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G22，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-160` | `complexity\|internal/cli/select.go\|selectMany` | 圈复杂度 | `internal/cli/select.go` | `selectMany` | 267 | 73/10 | +63（7.30x） | P1 | 3 可信 + 0 待核 caller / 10 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `FN-028` | `func_lines\|internal/cli/select.go\|selectMany` | 函数有效行数 | `internal/cli/select.go` | `selectMany` | 267 | 205/80 | +125（2.56x） | P1 | 3 可信 + 0 待核 caller / 10 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `ND-009` | `nesting_depth\|internal/cli/select.go\|selectMany` | 嵌套深度 | `internal/cli/select.go` | `selectMany` | 267 | 5/4 | +1（1.25x） | P1 | 3 可信 + 0 待核 caller / 10 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-159` | `complexity\|internal/cli/select.go\|selectOne` | 圈复杂度 | `internal/cli/select.go` | `selectOne` | 81 | 49/10 | +39（4.90x） | P1 | 4 可信 + 0 待核 caller / 10 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `FN-027` | `func_lines\|internal/cli/select.go\|selectOne` | 函数有效行数 | `internal/cli/select.go` | `selectOne` | 81 | 156/80 | +76（1.95x） | P1 | 4 可信 + 0 待核 caller / 10 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-146` | `complexity\|internal/cli/mcp_manager.go\|chatTUI.handleMCPManagerKey` | 圈复杂度 | `internal/cli/mcp_manager.go` | `chatTUI.handleMCPManagerKey` | 115 | 45/10 | +35（4.50x） | P1 | 0 可信 + 1 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `FN-026` | `func_lines\|internal/cli/mcp_manager.go\|chatTUI.handleMCPManagerKey` | 函数有效行数 | `internal/cli/mcp_manager.go` | `chatTUI.handleMCPManagerKey` | 115 | 130/80 | +50（1.63x） | P1 | 0 可信 + 1 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-104` | `complexity\|internal/cli/bot.go\|botDoctor` | 圈复杂度 | `internal/cli/bot.go` | `botDoctor` | 164 | 31/10 | +21（3.10x） | P2 | 1 可信 + 0 待核 caller / 4 可信 + 1 待核 callee / 52 unresolved | 待修改 |
| `FN-017` | `func_lines\|internal/cli/bot.go\|botDoctor` | 函数有效行数 | `internal/cli/bot.go` | `botDoctor` | 164 | 133/80 | +53（1.66x） | P2 | 1 可信 + 0 待核 caller / 4 可信 + 1 待核 callee / 52 unresolved | 待修改 |
| `CX-147` | `complexity\|internal/cli/mcp_manager.go\|chatTUI.buildMCPSnapshot` | 圈复杂度 | `internal/cli/mcp_manager.go` | `chatTUI.buildMCPSnapshot` | 298 | 16/10 | +6（1.60x） | P2 | 0 可信 + 3 待核 caller / 5 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-160 · 圈复杂度 · `selectMany`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-160 |
| Baseline key | `complexity\|internal/cli/select.go\|selectMany` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 70） |
| 文件 | `internal/cli/select.go` |
| 代码行 | 267 |
| 函数/方法/包 | `selectMany` |
| Baseline / 当前复测 / 阈值 | 73 / 73 / 10 |
| 超出 | +63，7.30x |
| 同一治理对象关联问题 | `FN-028`、`ND-009` |

#### 明确的问题描述

`selectMany` 的圈复杂度当前为 **73**，超过限制 **10**（+63，7.30x）。定义范围为 267-487 行；可见控制流热点：L270 `if err != nil {`；L333 `if n > 0 && end < n {`；L385 `if c {`；L422 `case len(k) >= 3 && k[0] == 27 && k[1] == '[' && k[2] == 'B':`；L454 `case k[0] == 3 || k[0] == 'q':`；L479 `if cur < len(filtered)-1 {`。CodeGraph 记录的下游职责候选主要涉及 渲染/输出（maxViewport、renderSearchBar）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function selectMany（internal/cli/select.go:267-487）
- **可信生产调用方（3）：** `selectEnabledProviders`（`internal/cli/cli.go:1120`，调用点 L1138/L1188，confidence=0.90，exact-match，2 条边）；`promptCustomProviderFromURL`（`internal/cli/cli.go:1499`，调用点 L1531，confidence=0.90，exact-match）；`promptAnthropicProviderFromURL`（`internal/cli/cli.go:1605`，调用点 L1637，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（10）：** `dim`（`internal/cli/style.go:49`，调用点 L306/L326/L334/L342，confidence=0.90，exact-match，5 条边）；`maxViewport`（`internal/cli/select.go:44`，调用点 L294/L356，confidence=0.90，exact-match，2 条边）；`filterMenuItems`（`internal/cli/select.go:61`，调用点 L402/L413，confidence=0.90，exact-match，2 条边）；`filterIndices`（`internal/cli/select.go:490`，调用点 L403/L414，confidence=0.90，exact-match，2 条边）；`bold`（`internal/cli/style.go:48`，调用点 L342/L345，confidence=0.90，exact-match，2 条边）；`accent`（`internal/cli/style.go:53`，调用点 L342/L345，confidence=0.90，exact-match，2 条边）；`termHeight`（`internal/cli/select.go:23`，调用点 L276，confidence=0.90，exact-match）；`fixedLines`（`internal/cli/select.go:34`，调用点 L356，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/select.go`、`internal/cli/cli.go`、`internal/cli/style.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `selectMany` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L270、L362、L400、L445、L479 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `selectMany` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-160、FN-028、ND-009 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/select.go|selectMany` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/select.go:267-487` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `selectMany` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G22 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-160`、`FN-028`、`ND-009` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/select.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/select.go|selectMany` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-028 · 函数有效行数 · `selectMany`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-028 |
| Baseline key | `func_lines\|internal/cli/select.go\|selectMany` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P1，治理对象得分 70） |
| 文件 | `internal/cli/select.go` |
| 代码行 | 267 |
| 函数/方法/包 | `selectMany` |
| Baseline / 当前复测 / 阈值 | 205 / 205 / 80 |
| 超出 | +125，2.56x |
| 同一治理对象关联问题 | `CX-160`、`ND-009` |

#### 明确的问题描述

`selectMany` 的有效代码行 **205**，超过限制 **80**（+125）。CodeGraph 定义物理范围为 267-487 行，并记录 10 个直接下游候选；候选职责分布为 渲染/输出（maxViewport、renderSearchBar）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function selectMany（internal/cli/select.go:267-487）
- **可信生产调用方（3）：** `selectEnabledProviders`（`internal/cli/cli.go:1120`，调用点 L1138/L1188，confidence=0.90，exact-match，2 条边）；`promptCustomProviderFromURL`（`internal/cli/cli.go:1499`，调用点 L1531，confidence=0.90，exact-match）；`promptAnthropicProviderFromURL`（`internal/cli/cli.go:1605`，调用点 L1637，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（10）：** `dim`（`internal/cli/style.go:49`，调用点 L306/L326/L334/L342，confidence=0.90，exact-match，5 条边）；`maxViewport`（`internal/cli/select.go:44`，调用点 L294/L356，confidence=0.90，exact-match，2 条边）；`filterMenuItems`（`internal/cli/select.go:61`，调用点 L402/L413，confidence=0.90，exact-match，2 条边）；`filterIndices`（`internal/cli/select.go:490`，调用点 L403/L414，confidence=0.90，exact-match，2 条边）；`bold`（`internal/cli/style.go:48`，调用点 L342/L345，confidence=0.90，exact-match，2 条边）；`accent`（`internal/cli/style.go:53`，调用点 L342/L345，confidence=0.90，exact-match，2 条边）；`termHeight`（`internal/cli/select.go:23`，调用点 L276，confidence=0.90，exact-match）；`fixedLines`（`internal/cli/select.go:34`，调用点 L356，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/select.go`、`internal/cli/cli.go`、`internal/cli/style.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `selectMany` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `selectMany` 的 205 个有效行按 渲染/输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L278 `search state` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-160、FN-028、ND-009 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/select.go|selectMany` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/select.go:267-487` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `selectMany` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G22 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-160`、`FN-028`、`ND-009` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/select.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/select.go|selectMany` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-009 · 嵌套深度 · `selectMany`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-009 |
| Baseline key | `nesting_depth\|internal/cli/select.go\|selectMany` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P3，治理对象得分 70） |
| 文件 | `internal/cli/select.go` |
| 代码行 | 267 |
| 函数/方法/包 | `selectMany` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-160`、`FN-028` |

#### 明确的问题描述

`selectMany` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L270 `if err != nil {`；L333 `if n > 0 && end < n {`；L385 `if c {`；L422 `case len(k) >= 3 && k[0] == 27 && k[1] == '[' && k[2] == 'B':`；L454 `case k[0] == 3 || k[0] == 'q':`；L479 `if cur < len(filtered)-1 {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function selectMany（internal/cli/select.go:267-487）
- **可信生产调用方（3）：** `selectEnabledProviders`（`internal/cli/cli.go:1120`，调用点 L1138/L1188，confidence=0.90，exact-match，2 条边）；`promptCustomProviderFromURL`（`internal/cli/cli.go:1499`，调用点 L1531，confidence=0.90，exact-match）；`promptAnthropicProviderFromURL`（`internal/cli/cli.go:1605`，调用点 L1637，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（10）：** `dim`（`internal/cli/style.go:49`，调用点 L306/L326/L334/L342，confidence=0.90，exact-match，5 条边）；`maxViewport`（`internal/cli/select.go:44`，调用点 L294/L356，confidence=0.90，exact-match，2 条边）；`filterMenuItems`（`internal/cli/select.go:61`，调用点 L402/L413，confidence=0.90，exact-match，2 条边）；`filterIndices`（`internal/cli/select.go:490`，调用点 L403/L414，confidence=0.90，exact-match，2 条边）；`bold`（`internal/cli/style.go:48`，调用点 L342/L345，confidence=0.90，exact-match，2 条边）；`accent`（`internal/cli/style.go:53`，调用点 L342/L345，confidence=0.90，exact-match，2 条边）；`termHeight`（`internal/cli/select.go:23`，调用点 L276，confidence=0.90，exact-match）；`fixedLines`（`internal/cli/select.go:34`，调用点 L356，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/select.go`、`internal/cli/cli.go`、`internal/cli/style.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `selectMany` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 从 L270、L362、L400、L445、L479 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-160、FN-028、ND-009 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/cli/select.go|selectMany` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/select.go:267-487` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `selectMany` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G22 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-160`、`FN-028`、`ND-009` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/select.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/cli/select.go|selectMany` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-159 · 圈复杂度 · `selectOne`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-159 |
| Baseline key | `complexity\|internal/cli/select.go\|selectOne` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 63） |
| 文件 | `internal/cli/select.go` |
| 代码行 | 81 |
| 函数/方法/包 | `selectOne` |
| Baseline / 当前复测 / 阈值 | 49 / 49 / 10 |
| 超出 | +39，4.90x |
| 同一治理对象关联问题 | `FN-027` |

#### 明确的问题描述

`selectOne` 的圈复杂度当前为 **49**，超过限制 **10**（+39，4.90x）。定义范围为 81-260 行；可见控制流热点：L84 `if err != nil {`；L131 `for i := scroll; i < end; i++ {`；L180 `if err != nil {`；L204 `if len(searchQuery) > 0 {`；L239 `case len(k) >= 3 && k[0] == 27 && k[1] == '[' && k[2] == 'A': // up`；L252 `if sel < len(filtered)-1 {`。CodeGraph 记录的下游职责候选主要涉及 渲染/输出（maxViewport、renderSearchBar）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function selectOne（internal/cli/select.go:81-260）
- **可信生产调用方（4）：** `pickSessionToResumeForActor`（`internal/cli/cli.go:1059`，调用点 L1085，confidence=0.90，exact-match）；`selectLanguage`（`internal/cli/cli.go:1096`，调用点 L1104，confidence=0.90，exact-match）；`promptCustomProvider`（`internal/cli/cli.go:1442`，调用点 L1443，confidence=0.90，exact-match）；`promptAnthropicProvider`（`internal/cli/cli.go:1548`，调用点 L1549，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（10）：** `dim`（`internal/cli/style.go:49`，调用点 L121/L137/L148/L156，confidence=0.90，exact-match，5 条边）；`maxViewport`（`internal/cli/select.go:44`，调用点 L107/L172，confidence=0.90，exact-match，2 条边）；`filterMenuItems`（`internal/cli/select.go:61`，调用点 L206/L217，confidence=0.90，exact-match，2 条边）；`filterIndices`（`internal/cli/select.go:490`，调用点 L207/L218，confidence=0.90，exact-match，2 条边）；`bold`（`internal/cli/style.go:48`，调用点 L156/L159，confidence=0.90，exact-match，2 条边）；`accent`（`internal/cli/style.go:53`，调用点 L156/L159，confidence=0.90，exact-match，2 条边）；`termHeight`（`internal/cli/select.go:23`，调用点 L90，confidence=0.90，exact-match）；`fixedLines`（`internal/cli/select.go:34`，调用点 L172，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `chatREPL`（`internal/cli/cli.go:652`，调用点 L680，confidence=0.90，exact-match，待源码复核）；`pickSessionToResume`（`internal/cli/cli.go:1055`，调用点 L1056，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/select.go`、`internal/cli/cli.go`、`internal/cli/style.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `selectOne` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L84、L142、L192、L232、L252 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `selectOne` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-159、FN-027 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/select.go|selectOne` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/select.go:81-260` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `selectOne` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G22 处理。
- 重点保护：协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-159`、`FN-027` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/select.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/select.go|selectOne` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-027 · 函数有效行数 · `selectOne`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-027 |
| Baseline key | `func_lines\|internal/cli/select.go\|selectOne` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P2，治理对象得分 63） |
| 文件 | `internal/cli/select.go` |
| 代码行 | 81 |
| 函数/方法/包 | `selectOne` |
| Baseline / 当前复测 / 阈值 | 156 / 156 / 80 |
| 超出 | +76，1.95x |
| 同一治理对象关联问题 | `CX-159` |

#### 明确的问题描述

`selectOne` 的有效代码行 **156**，超过限制 **80**（+76）。CodeGraph 定义物理范围为 81-260 行，并记录 10 个直接下游候选；候选职责分布为 渲染/输出（maxViewport、renderSearchBar）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function selectOne（internal/cli/select.go:81-260）
- **可信生产调用方（4）：** `pickSessionToResumeForActor`（`internal/cli/cli.go:1059`，调用点 L1085，confidence=0.90，exact-match）；`selectLanguage`（`internal/cli/cli.go:1096`，调用点 L1104，confidence=0.90，exact-match）；`promptCustomProvider`（`internal/cli/cli.go:1442`，调用点 L1443，confidence=0.90，exact-match）；`promptAnthropicProvider`（`internal/cli/cli.go:1548`，调用点 L1549，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（10）：** `dim`（`internal/cli/style.go:49`，调用点 L121/L137/L148/L156，confidence=0.90，exact-match，5 条边）；`maxViewport`（`internal/cli/select.go:44`，调用点 L107/L172，confidence=0.90，exact-match，2 条边）；`filterMenuItems`（`internal/cli/select.go:61`，调用点 L206/L217，confidence=0.90，exact-match，2 条边）；`filterIndices`（`internal/cli/select.go:490`，调用点 L207/L218，confidence=0.90，exact-match，2 条边）；`bold`（`internal/cli/style.go:48`，调用点 L156/L159，confidence=0.90，exact-match，2 条边）；`accent`（`internal/cli/style.go:53`，调用点 L156/L159，confidence=0.90，exact-match，2 条边）；`termHeight`（`internal/cli/select.go:23`，调用点 L90，confidence=0.90，exact-match）；`fixedLines`（`internal/cli/select.go:34`，调用点 L172，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `chatREPL`（`internal/cli/cli.go:652`，调用点 L680，confidence=0.90，exact-match，待源码复核）；`pickSessionToResume`（`internal/cli/cli.go:1055`，调用点 L1056，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/select.go`、`internal/cli/cli.go`、`internal/cli/style.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `selectOne` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `selectOne` 的 156 个有效行按 渲染/输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L92 `search state`、L126 `menu rows`、L141 `height stays constant`、L170 `previous frame don't linger.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-159、FN-027 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/select.go|selectOne` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/select.go:81-260` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `selectOne` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G22 处理。
- 重点保护：协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-159`、`FN-027` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/select.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/select.go|selectOne` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-146 · 圈复杂度 · `chatTUI.handleMCPManagerKey`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-146 |
| Baseline key | `complexity\|internal/cli/mcp_manager.go\|chatTUI.handleMCPManagerKey` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 58） |
| 文件 | `internal/cli/mcp_manager.go` |
| 代码行 | 115 |
| 函数/方法/包 | `chatTUI.handleMCPManagerKey` |
| Baseline / 当前复测 / 阈值 | 45 / 45 / 10 |
| 超出 | +35，4.50x |
| 同一治理对象关联问题 | `FN-026` |

#### 明确的问题描述

`chatTUI.handleMCPManagerKey` 的圈复杂度当前为 **45**，超过限制 **10**（+35，4.50x）。定义范围为 115-247 行；可见控制流热点：L117 `if p == nil {`；L145 `switch msg.String() {`；L170 `switch msg.String() {`；L192 `if p.mode > 0 {`；L218 `case "n":`；L240 `if p.confirm == 0 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::handleMCPManagerKey（internal/cli/mcp_manager.go:115-247）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L903，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `numberKeyIndex`（`internal/cli/mcp_manager.go:414`，调用点 L184/L202，confidence=0.90，exact-match，2 条边）；`mcpActionsFor`（`internal/cli/mcp_manager_view.go:235`，调用点 L169，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `mcpManager::selectedServer`（`internal/cli/mcp_manager.go:284`，调用点 L164，confidence=0.70，instance-method，待源码复核）；`chatTUI::buildMCPSnapshot`（`internal/cli/mcp_manager.go:298`，调用点 L155，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L903，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/mcp_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/mcp_manager.go`、`internal/cli/chat_tui.go`、`internal/cli/mcp_manager_view.go`、`internal/cli/mcp_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.handleMCPManagerKey` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L117、L150、L180、L209、L240 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `chatTUI.handleMCPManagerKey` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-146、FN-026 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/mcp_manager.go|chatTUI.handleMCPManagerKey` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/mcp_manager.go:115-247` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.handleMCPManagerKey` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G22 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-146`、`FN-026` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/mcp_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/mcp_manager.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/mcp_manager.go|chatTUI.handleMCPManagerKey` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-026 · 函数有效行数 · `chatTUI.handleMCPManagerKey`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-026 |
| Baseline key | `func_lines\|internal/cli/mcp_manager.go\|chatTUI.handleMCPManagerKey` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P2，治理对象得分 58） |
| 文件 | `internal/cli/mcp_manager.go` |
| 代码行 | 115 |
| 函数/方法/包 | `chatTUI.handleMCPManagerKey` |
| Baseline / 当前复测 / 阈值 | 130 / 130 / 80 |
| 超出 | +50，1.63x |
| 同一治理对象关联问题 | `CX-146` |

#### 明确的问题描述

`chatTUI.handleMCPManagerKey` 的有效代码行 **130**，超过限制 **80**（+50）。CodeGraph 定义物理范围为 115-247 行，并记录 4 个直接下游候选；候选职责分布为 CodeGraph 未提供足够的下游调用分类。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::handleMCPManagerKey（internal/cli/mcp_manager.go:115-247）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L903，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `numberKeyIndex`（`internal/cli/mcp_manager.go:414`，调用点 L184/L202，confidence=0.90，exact-match，2 条边）；`mcpActionsFor`（`internal/cli/mcp_manager_view.go:235`，调用点 L169，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `mcpManager::selectedServer`（`internal/cli/mcp_manager.go:284`，调用点 L164，confidence=0.70，instance-method，待源码复核）；`chatTUI::buildMCPSnapshot`（`internal/cli/mcp_manager.go:298`，调用点 L155，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L903，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/mcp_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/mcp_manager.go`、`internal/cli/chat_tui.go`、`internal/cli/mcp_manager_view.go`、`internal/cli/mcp_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.handleMCPManagerKey` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `chatTUI.handleMCPManagerKey` 的 130 个有效行按 输入、处理、输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-146、FN-026 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/mcp_manager.go|chatTUI.handleMCPManagerKey` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/mcp_manager.go:115-247` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.handleMCPManagerKey` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G22 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-146`、`FN-026` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/mcp_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/mcp_manager.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/mcp_manager.go|chatTUI.handleMCPManagerKey` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-104 · 圈复杂度 · `botDoctor`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-104 |
| Baseline key | `complexity\|internal/cli/bot.go\|botDoctor` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 55） |
| 文件 | `internal/cli/bot.go` |
| 代码行 | 164 |
| 函数/方法/包 | `botDoctor` |
| Baseline / 当前复测 / 阈值 | 31 / 31 / 10 |
| 超出 | +21，3.10x |
| 同一治理对象关联问题 | `FN-017` |

#### 明确的问题描述

`botDoctor` 的圈复杂度当前为 **31**，超过限制 **10**（+21，3.10x）。定义范围为 164-317 行；可见控制流热点：L168 `if err := fs.Parse(args); err != nil {`；L210 `if bc.QQ.AppID == "" {`；L234 `if mode == "" {`；L264 `for _, conn := range bc.Connections {`；L292 `if *jsonOut {`；L309 `if r.Detail != "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadBotCommandConfigForActor、NewProfilePathResolver、mathParser::Parse）、持久化/I-O（HasSavedAccountWithPaths、NewProfilePathResolver）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function botDoctor（internal/cli/bot.go:164-317）
- **可信生产调用方（1）：** `botCommand`（`internal/cli/bot.go:21`，调用点 L34，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `HasSavedAccountWithPaths`（`internal/bot/weixin/weixin_login.go:112`，调用点 L248，confidence=0.90，import）；`loadBotCommandConfigForActor`（`internal/cli/bot.go:356`，调用点 L174，confidence=0.90，exact-match）；`defaultCLIActorContext`（`internal/cli/cli.go:225`，调用点 L172，confidence=0.90，exact-match）；`NewProfilePathResolver`（`internal/config/paths.go:81`，调用点 L173，confidence=0.90，import）
- **待源码复核被调候选（1）：** `mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L168，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L122，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 52 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/cli`、`internal/bot`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/bot.go`、`internal/bot/weixin/weixin_login.go`、`internal/cli/cli.go`、`internal/config/paths.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `botDoctor` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L168、L220、L258、L280、L309 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `botDoctor` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-104、FN-017 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/bot.go|botDoctor` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/bot.go:164-317` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `botDoctor` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G22 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-104`、`FN-017` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/bot.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/bot.go|botDoctor` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-017 · 函数有效行数 · `botDoctor`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-017 |
| Baseline key | `func_lines\|internal/cli/bot.go\|botDoctor` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 55） |
| 文件 | `internal/cli/bot.go` |
| 代码行 | 164 |
| 函数/方法/包 | `botDoctor` |
| Baseline / 当前复测 / 阈值 | 133 / 133 / 80 |
| 超出 | +53，1.66x |
| 同一治理对象关联问题 | `CX-104` |

#### 明确的问题描述

`botDoctor` 的有效代码行 **133**，超过限制 **80**（+53）。CodeGraph 定义物理范围为 164-317 行，并记录 5 个直接下游候选；候选职责分布为 配置/解析（loadBotCommandConfigForActor、NewProfilePathResolver、mathParser::Parse）、持久化/I-O（HasSavedAccountWithPaths、NewProfilePathResolver）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function botDoctor（internal/cli/bot.go:164-317）
- **可信生产调用方（1）：** `botCommand`（`internal/cli/bot.go:21`，调用点 L34，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `HasSavedAccountWithPaths`（`internal/bot/weixin/weixin_login.go:112`，调用点 L248，confidence=0.90，import）；`loadBotCommandConfigForActor`（`internal/cli/bot.go:356`，调用点 L174，confidence=0.90，exact-match）；`defaultCLIActorContext`（`internal/cli/cli.go:225`，调用点 L172，confidence=0.90，exact-match）；`NewProfilePathResolver`（`internal/config/paths.go:81`，调用点 L173，confidence=0.90，import）
- **待源码复核被调候选（1）：** `mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L168，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L122，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 52 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/cli`、`internal/bot`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/bot.go`、`internal/bot/weixin/weixin_login.go`、`internal/cli/cli.go`、`internal/config/paths.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `botDoctor` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `botDoctor` 的 133 个有效行按 配置/解析、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L194 `基础检查`、L201 `QQ 检查`、L242 `微信检查`、L279 `Allowlist 检查` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-104、FN-017 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/bot.go|botDoctor` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/bot.go:164-317` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `botDoctor` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G22 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-104`、`FN-017` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/bot.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/bot.go|botDoctor` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-147 · 圈复杂度 · `chatTUI.buildMCPSnapshot`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-147 |
| Baseline key | `complexity\|internal/cli/mcp_manager.go\|chatTUI.buildMCPSnapshot` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 45） |
| 文件 | `internal/cli/mcp_manager.go` |
| 代码行 | 298 |
| 函数/方法/包 | `chatTUI.buildMCPSnapshot` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.buildMCPSnapshot` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 298-366 行；可见控制流热点：L302 `if err != nil {`；L314 `if m.host != nil {`；L327 `for _, f := range m.host.Failures() {`；L338 `for _, name := range m.host.ConnectingServers() {`；L350 `for _, p := range configuredEntries {`；L356 `case m.mcpDisabled[p.Name] || !p.ShouldAutoStart():`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（withMCPPluginConfig）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::buildMCPSnapshot（internal/cli/mcp_manager.go:298-366）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（3）：** `chatTUI::openMCPManager`（`internal/cli/mcp_manager.go:98`，调用点 L99，confidence=0.70，instance-method，待源码复核）；`chatTUI::refreshMCPManager`（`internal/cli/mcp_manager.go:107`，调用点 L111，confidence=0.70，instance-method，待源码复核）；`chatTUI::handleMCPManagerKey`（`internal/cli/mcp_manager.go:115`，调用点 L155，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（5）：** `withMCPPluginConfig`（`internal/cli/mcp_manager.go:368`，调用点 L322/L333/L344/L361，confidence=0.90，exact-match，4 条边）；`fallbackText`（`internal/cli/mcp_manager.go:422`，调用点 L317/L329，confidence=0.90，exact-match，2 条边）；`Host::Servers`（`internal/plugin/plugin.go:535`，调用点 L315，confidence=0.90，exact-match）；`Host::Failures`（`internal/plugin/plugin.go:562`，调用点 L327，confidence=0.90，exact-match）；`Host::ConnectingServers`（`internal/plugin/plugin.go:573`，调用点 L338，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L903，confidence=0.70，instance-method，待源码复核）；`chatTUI::removeSelectedMCP`（`internal/cli/mcp_manager_actions.go:101`，调用点 L121，confidence=0.70，instance-method，待源码复核）；`chatTUI::applyMCPMode`（`internal/cli/mcp_manager_actions.go:134`，调用点 L181，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`、`internal/plugin`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/mcp_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/mcp_manager.go`、`internal/plugin/plugin.go`、`internal/cli/mcp_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.buildMCPSnapshot` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L302、L315、L332、L350、L356 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `chatTUI.buildMCPSnapshot` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/mcp_manager.go|chatTUI.buildMCPSnapshot` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/mcp_manager.go:298-366` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.buildMCPSnapshot` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G22 处理。
- 重点保护：配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/mcp_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/mcp_manager.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/mcp_manager.go|chatTUI.buildMCPSnapshot` 下降或消失且无新增/恶化。
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
