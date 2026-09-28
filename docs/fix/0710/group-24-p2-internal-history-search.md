# G24 · internal/history · search 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G24 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/history`、`internal/agent` |
| 涉及文件 | `internal/history/search.go`<br>`internal/agent/compact.go` |
| 主要治理目标 | 围绕 `internal/history/search.go` 等治理对象降低复杂度/体量，统一处理 2 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | history / agent 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 6 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 24 组；无同文件组依赖 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 会话、状态或恢复语义；持久化、迁移或文件格式兼容；权限、安全或凭据边界；协议、序列化或流式事件兼容；返回值、错误语义和现有调用顺序；并发、取消或事件顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **46/100**，对象 `extractDocuments` 的分项为严重度 9.8、集中度 11.4、影响面 9.0、可执行性 16.0；最高超限 1.80x，文件内 6 条问题，CodeGraph 记录 1 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、嵌套深度，必须合并治理。

### 治理边界与组间关系

本组涉及 2 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-270` | `complexity\|internal/history/search.go\|extractDocuments` | 圈复杂度 | `internal/history/search.go` | `extractDocuments` | 381 | 18/10 | +8（1.80x） | P2 | 1 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `ND-012` | `nesting_depth\|internal/history/search.go\|extractDocuments` | 嵌套深度 | `internal/history/search.go` | `extractDocuments` | 381 | 5/4 | +1（1.25x） | P2 | 1 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-028` | `complexity\|internal/agent/compact.go\|*Agent.maybeCompact` | 圈复杂度 | `internal/agent/compact.go` | `*Agent.maybeCompact` | 84 | 15/10 | +5（1.50x） | P2 | 0 可信 + 1 待核 caller / 0 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-030` | `complexity\|internal/agent/compact.go\|keepIndexes` | 圈复杂度 | `internal/agent/compact.go` | `keepIndexes` | 400 | 11/10 | +1（1.10x） | P2 | 1 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-029` | `complexity\|internal/agent/compact.go\|*Agent.compact` | 圈复杂度 | `internal/agent/compact.go` | `*Agent.compact` | 189 | 12/10 | +2（1.20x） | P2 | 0 可信 + 0 待核 caller / 3 可信 + 5 待核 callee / 0 unresolved | 待修改 |
| `CX-267` | `complexity\|internal/history/search.go\|*Searcher.Search` | 圈复杂度 | `internal/history/search.go` | `*Searcher.Search` | 129 | 19/10 | +9（1.90x） | P2 | 0 可信 + 1 待核 caller / 8 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `FN-038` | `func_lines\|internal/history/search.go\|*Searcher.Search` | 函数有效行数 | `internal/history/search.go` | `*Searcher.Search` | 129 | 86/80 | +6（1.07x） | P2 | 0 可信 + 1 待核 caller / 8 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `CX-031` | `complexity\|internal/agent/compact.go\|*Agent.summarize` | 圈复杂度 | `internal/agent/compact.go` | `*Agent.summarize` | 601 | 13/10 | +3（1.30x） | P3 | 0 可信 + 0 待核 caller / 0 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-268` | `complexity\|internal/history/search.go\|*Searcher.Around` | 圈复杂度 | `internal/history/search.go` | `*Searcher.Around` | 222 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `CX-269` | `complexity\|internal/history/search.go\|listJSONL` | 圈复杂度 | `internal/history/search.go` | `listJSONL` | 336 | 11/10 | +1（1.10x） | P3 | 2 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-270 · 圈复杂度 · `extractDocuments`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-270 |
| Baseline key | `complexity\|internal/history/search.go\|extractDocuments` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 46） |
| 文件 | `internal/history/search.go` |
| 代码行 | 381 |
| 函数/方法/包 | `extractDocuments` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | `ND-012` |

#### 明确的问题描述

`extractDocuments` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 381-415 行；可见控制流热点：L383 `for i, msg := range msgs {`；L385 `case provider.RoleUser:`；L390 `if kinds[KindAssistantText] && strings.TrimSpace(msg.Content) != "" {`；L394 `for _, call := range msg.ToolCalls {`；L403 `if toolName != "" && msg.Name != toolName {`；L409 `if kinds[KindToolOutput] {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function extractDocuments（internal/history/search.go:381-415）
- **可信生产调用方（1）：** `Searcher::Search`（`internal/history/search.go:129`，调用点 L162，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `appendDoc`（`internal/history/search.go:417`，调用点 L387/L391/L399/L407，confidence=0.90，exact-match，5 条边）；`isToolError`（`internal/history/search.go:439`，调用点 L406，confidence=0.90，exact-match）；`stripComposePrefixes`（`internal/history/strip.go:12`，调用点 L387，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `historyTool::Execute`（`internal/history/tool.go:48`，调用点 L66，confidence=0.70，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/history`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/history/search_test.go`、`internal/history/tool_test.go`
- **需同步检查：** `internal/history/search.go`、`internal/history/strip.go`、`internal/history/search_test.go`、`internal/history/tool_test.go`

#### 待修改项

- [ ] 逐段核对 L383、L386、L393、L402、L409 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `extractDocuments` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-270、ND-012 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/history/search.go|extractDocuments` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/history/search.go:381-415` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `extractDocuments` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-270`、`ND-012` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/history/search_test.go`、`internal/history/tool_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/history`。
- [ ] 运行相关包 vet：`go vet ./internal/history`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/history/search.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/history/search.go|extractDocuments` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-012 · 嵌套深度 · `extractDocuments`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-012 |
| Baseline key | `nesting_depth\|internal/history/search.go\|extractDocuments` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 46） |
| 文件 | `internal/history/search.go` |
| 代码行 | 381 |
| 函数/方法/包 | `extractDocuments` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-270` |

#### 明确的问题描述

`extractDocuments` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L383 `for i, msg := range msgs {`；L385 `case provider.RoleUser:`；L390 `if kinds[KindAssistantText] && strings.TrimSpace(msg.Content) != "" {`；L394 `for _, call := range msg.ToolCalls {`；L403 `if toolName != "" && msg.Name != toolName {`；L409 `if kinds[KindToolOutput] {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function extractDocuments（internal/history/search.go:381-415）
- **可信生产调用方（1）：** `Searcher::Search`（`internal/history/search.go:129`，调用点 L162，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `appendDoc`（`internal/history/search.go:417`，调用点 L387/L391/L399/L407，confidence=0.90，exact-match，5 条边）；`isToolError`（`internal/history/search.go:439`，调用点 L406，confidence=0.90，exact-match）；`stripComposePrefixes`（`internal/history/strip.go:12`，调用点 L387，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `historyTool::Execute`（`internal/history/tool.go:48`，调用点 L66，confidence=0.70，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/history`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/history/search_test.go`、`internal/history/tool_test.go`
- **需同步检查：** `internal/history/search.go`、`internal/history/strip.go`、`internal/history/search_test.go`、`internal/history/tool_test.go`

#### 待修改项

- [ ] 从 L383、L386、L393、L402、L409 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-270、ND-012 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/history/search.go|extractDocuments` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/history/search.go:381-415` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `extractDocuments` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-270`、`ND-012` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/history/search_test.go`、`internal/history/tool_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/history`。
- [ ] 运行相关包 vet：`go vet ./internal/history`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/history/search.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/history/search.go|extractDocuments` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-028 · 圈复杂度 · `*Agent.maybeCompact`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-028 |
| Baseline key | `complexity\|internal/agent/compact.go\|*Agent.maybeCompact` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 45） |
| 文件 | `internal/agent/compact.go` |
| 代码行 | 84 |
| 函数/方法/包 | `*Agent.maybeCompact` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Agent.maybeCompact` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 84-135 行；可见控制流热点：L85 `if a.contextWindow <= 0 || u == nil || u.PromptTokens == 0 {`；L92 `if u.PromptTokens >= soft && u.PromptTokens < high && !a.softCompactNoticed {`；L104 `if a.compactStuck {`；L111 `if st, err := a.PruneStaleToolResults(); err == nil && st.Results > 0 {`；L119 `if err := a.compact(ctx, "auto", "", force); err != nil {`；L129 `if a.consecutiveCompacts >= 2 {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（Agent::PruneStaleToolResults）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::maybeCompact（internal/agent/compact.go:84-135）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L806/L816/L823/L835，confidence=0.70，instance-method，5 条边，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（2）：** `Agent::tokPerChar`（`internal/agent/compact.go:567`，调用点 L110，confidence=0.70，instance-method，待源码复核）；`Agent::PruneStaleToolResults`（`internal/agent/prune.go:29`，调用点 L111，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L806，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/compact_test.go`、`internal/agent/prune_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/compact.go`、`internal/agent/agent.go`、`internal/agent/prune.go`、`internal/agent/compact_test.go`、`internal/agent/prune_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.maybeCompact` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L85、L97、L111、L115、L129 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Agent.maybeCompact` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/compact.go|*Agent.maybeCompact` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/compact.go:84-135` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.maybeCompact` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/compact_test.go`、`internal/agent/prune_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/compact.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/compact.go|*Agent.maybeCompact` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-030 · 圈复杂度 · `keepIndexes`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-030 |
| Baseline key | `complexity\|internal/agent/compact.go\|keepIndexes` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 45） |
| 文件 | `internal/agent/compact.go` |
| 代码行 | 400 |
| 函数/方法/包 | `keepIndexes` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`keepIndexes` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 400-429 行；可见控制流热点：L403 `for i, m := range region {`；L410 `for i, m := range region {`；L415 `for i, m := range region {`；L416 `if !keep[i] {`；L420 `case provider.RoleTool:`；L424 `case provider.RoleAssistant:`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function keepIndexes（internal/agent/compact.go:400-429）
- **可信生产调用方（1）：** `Agent::partitionFold`（`internal/agent/compact.go:388`，调用点 L389，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `keepToolCallGroup`（`internal/agent/compact.go:431`，调用点 L422/L425，confidence=0.90，exact-match，2 条边）；`isCompactionSummary`（`internal/agent/compact.go:345`，调用点 L404，confidence=0.90，exact-match）；`shouldKeepMessage`（`internal/agent/compact.go:448`，调用点 L411，confidence=0.90，exact-match）；`findToolCaller`（`internal/agent/compact.go:477`，调用点 L421，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Agent::compact`（`internal/agent/compact.go:189`，调用点 L206，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/compact_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/compact.go`、`internal/agent/compact_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`

#### 待修改项

- [ ] 先冻结 `keepIndexes` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L403、L410、L416、L420、L424 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `keepIndexes` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/compact.go|keepIndexes` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/compact.go:400-429` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `keepIndexes` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/compact_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/compact.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/compact.go|keepIndexes` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-029 · 圈复杂度 · `*Agent.compact`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-029 |
| Baseline key | `complexity\|internal/agent/compact.go\|*Agent.compact` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 44） |
| 文件 | `internal/agent/compact.go` |
| 代码行 | 189 |
| 函数/方法/包 | `*Agent.compact` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Agent.compact` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 189-272 行；可见控制流热点：L192 `if !ok {`；L207 `if len(fold) == 0 {`；L221 `if a.hooks != nil {`；L222 `if hookInstr := a.hooks.PreCompact(ctx, trigger); hookInstr != "" {`；L231 `if a.archiveDir != "" {`；L245 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（archiveMessages）、渲染/输出（Agent::emitCompactionAborted）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::compact（internal/agent/compact.go:189-272）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `foldEconomics`（`internal/agent/compact.go:141`，调用点 L213，confidence=0.90，exact-match）；`mechanicalFoldDigest`（`internal/agent/compact.go:667`，调用点 L251，confidence=0.90，exact-match）；`archiveMessages`（`internal/agent/compact.go:722`，调用点 L232，confidence=0.90，exact-match）
- **待源码复核被调候选（5）：** `Agent::planCompaction`（`internal/agent/compact.go:505`，调用点 L191/L196，confidence=0.70，instance-method，2 条边，待源码复核）；`ToolHooks::PreCompact`（`internal/agent/agent.go:159`，调用点 L222，confidence=0.70，exact-match，待源码复核）；`Agent::emitCompactionAborted`（`internal/agent/compact.go:278`，调用点 L234，confidence=0.70，instance-method，待源码复核）；`Agent::partitionFold`（`internal/agent/compact.go:388`，调用点 L206，confidence=0.70，instance-method，待源码复核）；`Agent::summarizeWithRetry`（`internal/agent/compact.go:656`，调用点 L244，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/compact.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.compact` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L192、L207、L222、L231、L245 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Agent.compact` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/compact.go|*Agent.compact` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/compact.go:189-272` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.compact` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/compact.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/compact.go|*Agent.compact` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-267 · 圈复杂度 · `*Searcher.Search`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-267 |
| Baseline key | `complexity\|internal/history/search.go\|*Searcher.Search` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 44） |
| 文件 | `internal/history/search.go` |
| 代码行 | 129 |
| 函数/方法/包 | `*Searcher.Search` |
| Baseline / 当前复测 / 阈值 | 19 / 19 / 10 |
| 超出 | +9，1.90x |
| 同一治理对象关联问题 | `FN-038` |

#### 明确的问题描述

`*Searcher.Search` 的圈复杂度当前为 **19**，超过限制 **10**（+9，1.90x）。定义范围为 129-219 行；可见控制流热点：L131 `if query == "" {`；L144 `if err != nil {`；L159 `if err != nil {`；L173 `for term := range docs[i].counts {`；L188 `if score <= 0 {`；L215 `if len(hits) > limit {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeKinds、loadMessages、normalizeScope）、持久化/I-O（ArchiveSink::Err）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Searcher::Search（internal/history/search.go:129-219）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `historyTool::Execute`（`internal/history/tool.go:48`，调用点 L66，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（8）：** `normalizeKinds`（`internal/history/search.go:274`，调用点 L143，confidence=0.90，exact-match）；`loadMessages`（`internal/history/search.go:373`，调用点 L158，confidence=0.90，exact-match）；`extractDocuments`（`internal/history/search.go:381`，调用点 L162，confidence=0.90，exact-match）；`sessionID`（`internal/history/search.go:446`，调用点 L194，confidence=0.90，exact-match）；`BM25Score`（`internal/retrieval/bm25.go:67`，调用点 L187，confidence=0.90，import）；`KeepTopRelativeScore`（`internal/retrieval/bm25.go:111`，调用点 L212，confidence=0.90，import）；`QueryTerms`（`internal/retrieval/bm25.go:131`，调用点 L134，confidence=0.90，import）；`MakeSnippet`（`internal/retrieval/bm25.go:140`，调用点 L200，confidence=0.90，import）
- **待源码复核被调候选（4）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L155，confidence=0.70，instance-method，待源码复核）；`normalizeScope`（`internal/history/search.go:263`，调用点 L138，confidence=0.70，exact-match，待源码复核）；`Searcher::sources`（`internal/history/search.go:294`，调用点 L149，confidence=0.70，instance-method，待源码复核）；`clamp`（`internal/history/search.go:484`，调用点 L142，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `historyTool::Execute`（`internal/history/tool.go:48`，调用点 L66，confidence=0.70，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/history`、`internal/retrieval`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/history/search_test.go`
- **同目录测试候选：** `internal/history/tool_test.go`
- **需同步检查：** `internal/history/search.go`、`internal/history/tool.go`、`internal/history/search_test.go`、`internal/history/tool_test.go`

#### 待修改项

- [ ] 逐段核对 L131、L150、L170、L186、L215 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Searcher.Search` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-267、FN-038 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/history/search.go|*Searcher.Search` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/history/search.go:129-219` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Searcher.Search` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-267`、`FN-038` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/history/search_test.go`。
- [ ] 运行相关包测试：`go test ./internal/history`。
- [ ] 运行相关包 vet：`go vet ./internal/history`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/history/search.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/history/search.go|*Searcher.Search` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-038 · 函数有效行数 · `*Searcher.Search`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-038 |
| Baseline key | `func_lines\|internal/history/search.go\|*Searcher.Search` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 44） |
| 文件 | `internal/history/search.go` |
| 代码行 | 129 |
| 函数/方法/包 | `*Searcher.Search` |
| Baseline / 当前复测 / 阈值 | 86 / 86 / 80 |
| 超出 | +6，1.07x |
| 同一治理对象关联问题 | `CX-267` |

#### 明确的问题描述

`*Searcher.Search` 的有效代码行 **86**，超过限制 **80**（+6）。CodeGraph 定义物理范围为 129-219 行，并记录 12 个直接下游候选；候选职责分布为 配置/解析（normalizeKinds、loadMessages、normalizeScope）、持久化/I-O（ArchiveSink::Err）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Searcher::Search（internal/history/search.go:129-219）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `historyTool::Execute`（`internal/history/tool.go:48`，调用点 L66，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（8）：** `normalizeKinds`（`internal/history/search.go:274`，调用点 L143，confidence=0.90，exact-match）；`loadMessages`（`internal/history/search.go:373`，调用点 L158，confidence=0.90，exact-match）；`extractDocuments`（`internal/history/search.go:381`，调用点 L162，confidence=0.90，exact-match）；`sessionID`（`internal/history/search.go:446`，调用点 L194，confidence=0.90，exact-match）；`BM25Score`（`internal/retrieval/bm25.go:67`，调用点 L187，confidence=0.90，import）；`KeepTopRelativeScore`（`internal/retrieval/bm25.go:111`，调用点 L212，confidence=0.90，import）；`QueryTerms`（`internal/retrieval/bm25.go:131`，调用点 L134，confidence=0.90，import）；`MakeSnippet`（`internal/retrieval/bm25.go:140`，调用点 L200，confidence=0.90，import）
- **待源码复核被调候选（4）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L155，confidence=0.70，instance-method，待源码复核）；`normalizeScope`（`internal/history/search.go:263`，调用点 L138，confidence=0.70，exact-match，待源码复核）；`Searcher::sources`（`internal/history/search.go:294`，调用点 L149，confidence=0.70，instance-method，待源码复核）；`clamp`（`internal/history/search.go:484`，调用点 L142，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `historyTool::Execute`（`internal/history/tool.go:48`，调用点 L66，confidence=0.70，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/history`、`internal/retrieval`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/history/search_test.go`
- **同目录测试候选：** `internal/history/tool_test.go`
- **需同步检查：** `internal/history/search.go`、`internal/history/tool.go`、`internal/history/search_test.go`、`internal/history/tool_test.go`

#### 待修改项

- [ ] 把 `*Searcher.Search` 的 86 个有效行按 配置/解析、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-267、FN-038 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/history/search.go|*Searcher.Search` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/history/search.go:129-219` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Searcher.Search` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-267`、`FN-038` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/history/search_test.go`。
- [ ] 运行相关包测试：`go test ./internal/history`。
- [ ] 运行相关包 vet：`go vet ./internal/history`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/history/search.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/history/search.go|*Searcher.Search` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-031 · 圈复杂度 · `*Agent.summarize`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-031 |
| Baseline key | `complexity\|internal/agent/compact.go\|*Agent.summarize` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/agent/compact.go` |
| 代码行 | 601 |
| 函数/方法/包 | `*Agent.summarize` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Agent.summarize` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 601-651 行；可见控制流热点：L605 `if strings.TrimSpace(instructions) != "" {`；L624 `if usage != nil && usage.TotalTokens > 0 {`；L630 `case <-ctx.Done():`；L633 `if !ok {`；L642 `case provider.ChunkText:`；L646 `case provider.ChunkError:`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（ArchiveSink::Err）、渲染/输出（renderTranscript）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::summarize（internal/agent/compact.go:601-651）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（2）：** `renderTranscript`（`internal/agent/compact.go:676`，调用点 L611，confidence=0.70，exact-match，待源码复核）；`ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L631，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/agent`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/compact.go`、`internal/automation/archive_sink.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.summarize` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L605、L628、L632、L641、L646 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Agent.summarize` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/compact.go|*Agent.summarize` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/compact.go:601-651` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.summarize` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/compact.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/compact.go|*Agent.summarize` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-268 · 圈复杂度 · `*Searcher.Around`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-268 |
| Baseline key | `complexity\|internal/history/search.go\|*Searcher.Around` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/history/search.go` |
| 代码行 | 222 |
| 函数/方法/包 | `*Searcher.Around` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Searcher.Around` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 222-261 行；可见控制流热点：L224 `if path == "" {`；L230 `if !s.allowedPath(path) {`；L236 `if err := ctx.Err(); err != nil {`；L240 `if err != nil {`；L249 `if start < 0 {`；L257 `for i := start; i < end; i++ {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadMessages）、持久化/I-O（ArchiveSink::Err）、渲染/输出（renderMessage）、权限/安全（Searcher::allowedPath）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Searcher::Around（internal/history/search.go:222-261）
- **可信生产调用方（1）：** `historyTool::Execute`（`internal/history/tool.go:48`，调用点 L81，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `loadMessages`（`internal/history/search.go:373`，调用点 L239，confidence=0.90，exact-match）；`renderMessage`（`internal/history/search.go:451`，调用点 L258，confidence=0.90，exact-match）
- **待源码复核被调候选（4）：** `clamp`（`internal/history/search.go:484`，调用点 L246/L247，confidence=0.70，exact-match，2 条边，待源码复核）；`ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L236，confidence=0.70，instance-method，待源码复核）；`Searcher::visiblePath`（`internal/history/search.go:494`，调用点 L233，confidence=0.70，instance-method，待源码复核）；`Searcher::allowedPath`（`internal/history/search.go:545`，调用点 L230，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `historyTool::Execute`（`internal/history/tool.go:48`，调用点 L81，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/history`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/history/search_test.go`
- **同目录测试候选：** `internal/history/tool_test.go`
- **需同步检查：** `internal/history/search.go`、`internal/history/tool.go`、`internal/automation/archive_sink.go`、`internal/history/search_test.go`、`internal/history/tool_test.go`

#### 待修改项

- [ ] 逐段核对 L224、L230、L240、L249、L257 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Searcher.Around` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/history/search.go|*Searcher.Around` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/history/search.go:222-261` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Searcher.Around` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/history/search_test.go`。
- [ ] 运行相关包测试：`go test ./internal/history`。
- [ ] 运行相关包 vet：`go vet ./internal/history`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/history/search.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/history/search.go|*Searcher.Around` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-269 · 圈复杂度 · `listJSONL`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-269 |
| Baseline key | `complexity\|internal/history/search.go\|listJSONL` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/history/search.go` |
| 代码行 | 336 |
| 函数/方法/包 | `listJSONL` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`listJSONL` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 336-371 行；可见控制流热点：L337 `if strings.TrimSpace(dir) == "" {`；L341 `if !ok {`；L349 `for _, entry := range entries {`；L350 `if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {`；L358 `if !underResolvedRoot(path, realDir) {`；L361 `if visible != nil && !visible(path) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（resolvedRoot、underResolvedRoot）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function listJSONL（internal/history/search.go:336-371）
- **可信生产调用方（2）：** `appendSessionSources`（`internal/history/search.go:311`，调用点 L312/L314，confidence=0.90，exact-match，2 条边）；`Searcher::sources`（`internal/history/search.go:294`，调用点 L300，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `resolvedRoot`（`internal/history/search.go:566`，调用点 L340，confidence=0.90，exact-match）；`underResolvedRoot`（`internal/history/search.go:581`，调用点 L358，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/history`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/history/search_test.go`、`internal/history/tool_test.go`
- **需同步检查：** `internal/history/search.go`、`internal/history/search_test.go`、`internal/history/tool_test.go`

#### 待修改项

- [ ] 逐段核对 L337、L345、L350、L354、L361 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `listJSONL` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/history/search.go|listJSONL` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/history/search.go:336-371` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `listJSONL` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/history/search_test.go`、`internal/history/tool_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/history`。
- [ ] 运行相关包 vet：`go vet ./internal/history`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/history/search.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/history/search.go|listJSONL` 下降或消失且无新增/恶化。
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
