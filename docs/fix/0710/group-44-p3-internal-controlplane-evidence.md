# G44 · internal/controlplane · evidence 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G44 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/controlplane`、`internal/agent`、`internal/evidence` |
| 涉及文件 | `internal/controlplane/policy_nodes/weighting.go`<br>`internal/evidence/evidence.go`<br>`internal/agent/testutil/mock_provider.go`<br>`internal/controlplane/arbitration/conflict_resolver.go`<br>`internal/controlplane/consensus/consensus.go` |
| 主要治理目标 | 围绕 `internal/evidence/evidence.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | controlplane / agent / evidence 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 8 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 44 组；先于 G47 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 返回值、错误语义和现有调用顺序；会话、状态或恢复语义；并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **39/100**，对象 `dynamicNodeWeight` 的分项为严重度 7.2、集中度 3.6、影响面 8.4、可执行性 20.0；最高超限 1.90x，文件内 2 条问题，CodeGraph 记录 1 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 5 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G47，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-241` | `complexity\|internal/controlplane/policy_nodes/weighting.go\|dynamicNodeWeight` | 圈复杂度 | `internal/controlplane/policy_nodes/weighting.go` | `dynamicNodeWeight` | 53 | 19/10 | +9（1.90x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `CX-252` | `complexity\|internal/evidence/evidence.go\|hasFailedCompleteStepRecoveryForTodo` | 圈复杂度 | `internal/evidence/evidence.go` | `hasFailedCompleteStepRecoveryForTodo` | 467 | 15/10 | +5（1.50x） | P3 | 1 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-256` | `complexity\|internal/evidence/evidence.go\|matchTodoStep` | 圈复杂度 | `internal/evidence/evidence.go` | `matchTodoStep` | 848 | 13/10 | +3（1.30x） | P3 | 5 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-056` | `complexity\|internal/agent/testutil/mock_provider.go\|*MockProvider.Stream` | 圈复杂度 | `internal/agent/testutil/mock_provider.go` | `*MockProvider.Stream` | 71 | 14/10 | +4（1.40x） | P3 | 0 可信 + 0 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-238` | `complexity\|internal/controlplane/arbitration/conflict_resolver.go\|resolveConflicts` | 圈复杂度 | `internal/controlplane/arbitration/conflict_resolver.go` | `resolveConflicts` | 10 | 17/10 | +7（1.70x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `FL-013` | `file_lines\|internal/evidence/evidence.go\|` | 文件有效行数 | `internal/evidence/evidence.go` | `internal/evidence/evidence.go` | 1 | 815/800 | +15（1.02x） | P3 | 5 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-253` | `complexity\|internal/evidence/evidence.go\|PathsProvenInSession` | 圈复杂度 | `internal/evidence/evidence.go` | `PathsProvenInSession` | 568 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-239` | `complexity\|internal/controlplane/consensus/consensus.go\|Aggregate` | 圈复杂度 | `internal/controlplane/consensus/consensus.go` | `Aggregate` | 24 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-240` | `complexity\|internal/controlplane/policy_nodes/weighting.go\|LearnControlGraph` | 圈复杂度 | `internal/controlplane/policy_nodes/weighting.go` | `LearnControlGraph` | 36 | 14/10 | +4（1.40x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-255` | `complexity\|internal/evidence/evidence.go\|hasSuccessfulCompleteStepForTodo` | 圈复杂度 | `internal/evidence/evidence.go` | `hasSuccessfulCompleteStepForTodo` | 796 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-241 · 圈复杂度 · `dynamicNodeWeight`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-241 |
| Baseline key | `complexity\|internal/controlplane/policy_nodes/weighting.go\|dynamicNodeWeight` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/controlplane/policy_nodes/weighting.go` |
| 代码行 | 53 |
| 函数/方法/包 | `dynamicNodeWeight` |
| Baseline / 当前复测 / 阈值 | 19 / 19 / 10 |
| 超出 | +9，1.90x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`dynamicNodeWeight` 的圈复杂度当前为 **19**，超过限制 **10**（+9，1.90x）。定义范围为 53-95 行；可见控制流热点：L59 `if st.Stable {`；L65 `switch node.Type() {`；L71 `case controlgraph.NodeExploration:`；L75 `if st.Oscillating || len(st.SemanticShift) > 0 {`；L82 `if st.Unstable {`；L86 `if len(st.SemanticShift) > 0 || st.RecentHardDrifts > 0 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function dynamicNodeWeight（internal/controlplane/policy_nodes/weighting.go:53-95）
- **可信生产调用方（1）：** `ApplyDynamicWeights`（`internal/controlplane/policy_nodes/weighting.go:26`，调用点 L30，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `recentSuccessRate`（`internal/controlplane/policy_nodes/weighting.go:97`，调用点 L56，confidence=0.90，exact-match）
- **待源码复核被调候选（4）：** `clamp`（`internal/controlplane/policy_nodes/weighting.go:105`，调用点 L92/L93，confidence=0.70，exact-match，2 条边，待源码复核）；`node::Type`（`internal/controlplane/controllers/nodes.go:14`，调用点 L65，confidence=0.85，qualified-name，待源码复核）；`node::Weight`（`internal/controlplane/controllers/nodes.go:15`，调用点 L54，confidence=0.85，qualified-name，待源码复核）；`node::Reliability`（`internal/controlplane/controllers/nodes.go:16`，调用点 L55，confidence=0.85，qualified-name，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/controlplane`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/controlplane/policy_nodes/weighting.go`、`internal/controlplane/controllers/nodes.go`

#### 待修改项

- [ ] 逐段核对 L59、L66、L72、L79、L86 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `dynamicNodeWeight` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/controlplane/policy_nodes/weighting.go|dynamicNodeWeight` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/controlplane/policy_nodes/weighting.go:53-95` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `dynamicNodeWeight` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G47 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 补充或确认覆盖当前职责、错误分支和边界条件的聚焦测试；不得仅依赖全量测试。
- [ ] 运行相关包测试：`go test ./internal/controlplane/policy_nodes`。
- [ ] 运行相关包 vet：`go vet ./internal/controlplane/policy_nodes`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/controlplane/policy_nodes/weighting.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/controlplane/policy_nodes/weighting.go|dynamicNodeWeight` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-252 · 圈复杂度 · `hasFailedCompleteStepRecoveryForTodo`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-252 |
| Baseline key | `complexity\|internal/evidence/evidence.go\|hasFailedCompleteStepRecoveryForTodo` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/evidence/evidence.go` |
| 代码行 | 467 |
| 函数/方法/包 | `hasFailedCompleteStepRecoveryForTodo` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`hasFailedCompleteStepRecoveryForTodo` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 467-493 行；可见控制流热点：L468 `for i := baseline + 1; i < len(receipts); i++ {`；L470 `if r.Success || r.ToolName != "complete_step" || strings.TrimSpace(r.Step) == "" || !r.StepProof {`；L476 `if r.TodoStep != nil && r.TodoStep.Found {`；L477 `if index < 1 || index > len(current) {`；L483 `if !todoContentRelates(current[index-1], *r.TodoStep) {`；L488 `if match.Found && match.Index == index {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function hasFailedCompleteStepRecoveryForTodo（internal/evidence/evidence.go:467-493）
- **可信生产调用方（1）：** `Ledger::UnverifiedCompletedTodos`（`internal/evidence/evidence.go:416`，调用点 L453，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `hasSuccessfulProgressBeforeReceipt`（`internal/evidence/evidence.go:497`，调用点 L473，confidence=0.90，exact-match）；`sameTodoMatch`（`internal/evidence/evidence.go:831`，调用点 L480，confidence=0.90，exact-match）；`todoContentRelates`（`internal/evidence/evidence.go:839`，调用点 L483，confidence=0.90，exact-match）；`matchTodoStep`（`internal/evidence/evidence.go:848`，调用点 L487，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/evidence`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`
- **需同步检查：** `internal/evidence/evidence.go`、`internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`

#### 待修改项

- [ ] 逐段核对 L468、L473、L477、L480、L488 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `hasFailedCompleteStepRecoveryForTodo` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/evidence/evidence.go|hasFailedCompleteStepRecoveryForTodo` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/evidence/evidence.go:467-493` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `hasFailedCompleteStepRecoveryForTodo` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G47 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/evidence`。
- [ ] 运行相关包 vet：`go vet ./internal/evidence`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/evidence/evidence.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/evidence/evidence.go|hasFailedCompleteStepRecoveryForTodo` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-256 · 圈复杂度 · `matchTodoStep`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-256 |
| Baseline key | `complexity\|internal/evidence/evidence.go\|matchTodoStep` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/evidence/evidence.go` |
| 代码行 | 848 |
| 函数/方法/包 | `matchTodoStep` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`matchTodoStep` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 848-875 行；可见控制流热点：L849 `if n, ok := parseStepIndex(normalizeStepText(step)); ok && n >= 1 && n <= len(todos) {`；L853 `for i, t := range todos {`；L854 `if sameStepText(step, t.Content) || sameStepText(step, t.ActiveForm) {`；L863 `if stepTextContains(norm, normalizeStepText(t.Content)) || stepTextContains(norm, normalizeStepText…`；L864 `if found >= 0 && found != i {`；L870 `if found >= 0 {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeStepText、parseStepIndex）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function matchTodoStep（internal/evidence/evidence.go:848-875）
- **可信生产调用方（5）：** `MatchStep`（`internal/evidence/evidence.go:310`，调用点 L311，confidence=0.90，exact-match）；`Ledger::MatchLatestTodoStep`（`internal/evidence/evidence.go:378`，调用点 L390，confidence=0.90，exact-match）；`hasFailedCompleteStepRecoveryForTodo`（`internal/evidence/evidence.go:467`，调用点 L487，confidence=0.90，exact-match）；`hasSuccessfulCompleteStepForTodo`（`internal/evidence/evidence.go:796`，调用点 L812，confidence=0.90，exact-match）；`latestTodoStep`（`internal/evidence/evidence.go:820`，调用点 L826，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `normalizeStepText`（`internal/evidence/evidence.go:886`，调用点 L849/L860/L863，confidence=0.90，exact-match，4 条边）；`sameStepText`（`internal/evidence/evidence.go:897`，调用点 L854，confidence=0.90，exact-match，2 条边）；`stepTextContains`（`internal/evidence/evidence.go:904`，调用点 L863，confidence=0.90，exact-match，2 条边）；`parseStepIndex`（`internal/evidence/evidence.go:877`，调用点 L849，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/evidence`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`
- **需同步检查：** `internal/evidence/evidence.go`、`internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`

#### 待修改项

- [ ] 逐段核对 L849、L854、L862、L864、L870 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `matchTodoStep` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/evidence/evidence.go|matchTodoStep` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/evidence/evidence.go:848-875` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `matchTodoStep` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G47 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/evidence`。
- [ ] 运行相关包 vet：`go vet ./internal/evidence`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/evidence/evidence.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/evidence/evidence.go|matchTodoStep` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-056 · 圈复杂度 · `*MockProvider.Stream`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-056 |
| Baseline key | `complexity\|internal/agent/testutil/mock_provider.go\|*MockProvider.Stream` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/agent/testutil/mock_provider.go` |
| 代码行 | 71 |
| 函数/方法/包 | `*MockProvider.Stream` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*MockProvider.Stream` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 71-131 行；可见控制流热点：L72 `if err := ctx.Err(); err != nil {`；L91 `if len(t.Chunks) > 0 {`；L97 `if t.Text != "" {`；L107 `if t.ChunkError != nil {`；L118 `if err := ctx.Err(); err != nil {`；L126 `case ch <- c:`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（ArchiveSink::Err）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method MockProvider::Stream（internal/agent/testutil/mock_provider.go:71-131）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L72/L118/L124，confidence=0.70，instance-method，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/agent`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/testutil/mock_provider_test.go`
- **需同步检查：** `internal/agent/testutil/mock_provider.go`、`internal/automation/archive_sink.go`、`internal/agent/testutil/mock_provider_test.go`

#### 待修改项

- [ ] 先冻结 `*MockProvider.Stream` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L72、L91、L104、L118、L126 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*MockProvider.Stream` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/testutil/mock_provider.go|*MockProvider.Stream` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/testutil/mock_provider.go:71-131` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*MockProvider.Stream` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G47 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/testutil/mock_provider_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent/testutil`。
- [ ] 运行相关包 race：`go test -race ./internal/agent/testutil`。
- [ ] 运行相关包 vet：`go vet ./internal/agent/testutil`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/testutil/mock_provider.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/testutil/mock_provider.go|*MockProvider.Stream` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-238 · 圈复杂度 · `resolveConflicts`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-238 |
| Baseline key | `complexity\|internal/controlplane/arbitration/conflict_resolver.go\|resolveConflicts` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 37） |
| 文件 | `internal/controlplane/arbitration/conflict_resolver.go` |
| 代码行 | 10 |
| 函数/方法/包 | `resolveConflicts` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`resolveConflicts` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 10-92 行；可见控制流热点：L19 `if st.Oscillating {`；L28 `if st.HasDrift || st.Unstable || len(st.SemanticShift) > 0 {`；L48 `if len(st.SemanticShift) > 0 {`；L70 `switch decision.Action {`；L78 `case controlgraph.ActionDampen:`；L87 `if decision.Gain <= 0 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function resolveConflicts（internal/controlplane/arbitration/conflict_resolver.go:10-92）
- **可信生产调用方（1）：** `Arbitrate`（`internal/controlplane/arbitration/consensus.go:13`，调用点 L32，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `safeDecision`（`internal/controlplane/arbitration/consensus.go:61`，调用点 L29，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `round`（`internal/controlplane/arbitration/consensus.go:104`，调用点 L30，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/controlplane`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/controlplane/arbitration/conflict_resolver.go`、`internal/controlplane/arbitration/consensus.go`

#### 待修改项

- [ ] 逐段核对 L19、L33、L62、L75、L87 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `resolveConflicts` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/controlplane/arbitration/conflict_resolver.go|resolveConflicts` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/controlplane/arbitration/conflict_resolver.go:10-92` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `resolveConflicts` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G47 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 补充或确认覆盖当前职责、错误分支和边界条件的聚焦测试；不得仅依赖全量测试。
- [ ] 运行相关包测试：`go test ./internal/controlplane/arbitration`。
- [ ] 运行相关包 vet：`go vet ./internal/controlplane/arbitration`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/controlplane/arbitration/conflict_resolver.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/controlplane/arbitration/conflict_resolver.go|resolveConflicts` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FL-013 · 文件有效行数 · `internal/evidence/evidence.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-013 |
| Baseline key | `file_lines\|internal/evidence/evidence.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/evidence/evidence.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/evidence/evidence.go` |
| Baseline / 当前复测 / 阈值 | 815 / 815 / 800 |
| 超出 | +15，1.02x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/evidence/evidence.go` 当前有效代码行 **815**，超过限制 **800**（+15）。CodeGraph 在文件中索引到的主要大符号包括 Ledger::UnverifiedCompletedTodos@416-465、completeStepHasProof@718-756、ReceiptFromToolCall@610-638、PathsProvenInSession@568-595、matchTodoStep@848-875、hasFailedCompleteStepRecoveryForTodo@467-493；文件同时承载的调用影响覆盖 5 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 58 个函数/方法节点。
- **可信生产调用方（5）：** `verifyStepEvidence`（`internal/tool/builtin/completestep.go:144`，调用点 L176/L169/L177/L156，confidence=0.90，instance-method，8 条边）；`Agent::executeOneWithRegistry`（`internal/agent/agent.go:1826`，调用点 L1942/L1948/L1943/L1949，confidence=0.90，import，6 条边）；`Agent::finalReadinessCheck`（`internal/agent/agent.go:932`，调用点 L967/L939/L949/L943，confidence=0.90，exact-match，6 条边）；`historyTodoArgsWithCompleteSteps`（`desktop/app.go:3890`，调用点 L3902/L3915/L3916，confidence=0.90，import，3 条边）；`Agent::rebuildTodoState`（`internal/agent/agent.go:1119`，调用点 L1128/L1144/L1145，confidence=0.90，import，3 条边）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `CommandMatches`（`internal/evidence/commandmatch.go:10`，调用点 L214/L102/L119，confidence=0.90，exact-match，3 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/evidence`、`internal/tool`、`internal/agent`、`desktop`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/evidence/evidence_test.go`、`internal/tool/builtin/todo_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/evidence/readiness_test.go`
- **同目录测试候选：** `internal/evidence/commandmatch_test.go`
- **需同步检查：** `internal/evidence/evidence.go`、`internal/tool/builtin/completestep.go`、`internal/agent/agent.go`、`desktop/app.go`、`internal/evidence/commandmatch.go`、`internal/evidence/evidence_test.go`、`internal/tool/builtin/todo_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/evidence/readiness_test.go`

#### 待修改项

- [ ] 围绕主要符号 `Ledger::UnverifiedCompletedTodos`、`completeStepHasProof`、`ReceiptFromToolCall`、`PathsProvenInSession`、`matchTodoStep` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-250、CX-251、CX-252、CX-253、CX-254、CX-255、CX-256。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/evidence/evidence.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/evidence/evidence.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/evidence/evidence.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G47 处理。
- 重点保护：并发、取消或事件顺序；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/evidence/evidence_test.go`、`internal/tool/builtin/todo_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/evidence/readiness_test.go`。
- [ ] 运行相关包测试：`go test ./internal/evidence`。
- [ ] 运行相关包 race：`go test -race ./internal/evidence`。
- [ ] 运行相关包 vet：`go vet ./internal/evidence`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/evidence/evidence.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/evidence/evidence.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-253 · 圈复杂度 · `PathsProvenInSession`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-253 |
| Baseline key | `complexity\|internal/evidence/evidence.go\|PathsProvenInSession` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/evidence/evidence.go` |
| 代码行 | 568 |
| 函数/方法/包 | `PathsProvenInSession` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`PathsProvenInSession` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 568-595 行；可见控制流热点：L570 `if len(wanted) == 0 {`；L575 `for _, msg := range msgs {`；L577 `if failed[tc.ID] {`；L581 `if wantWrite && !r.Write {`；L587 `for _, p := range normalizePaths(r.Paths) {`；L588 `if _, ok := wanted[p]; ok {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizePaths）、状态与控制（pathSet）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function PathsProvenInSession（internal/evidence/evidence.go:568-595）
- **可信生产调用方（1）：** `verifyPathsFromSession`（`internal/tool/builtin/completestep.go:316`，调用点 L321，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `normalizePaths`（`internal/evidence/evidence.go:928`，调用点 L569/L587，confidence=0.90，exact-match，2 条边）；`failedSessionCallIDs`（`internal/evidence/evidence.go:597`，调用点 L573，confidence=0.90，exact-match）；`ReceiptFromToolCall`（`internal/evidence/evidence.go:610`，调用点 L580，confidence=0.90，exact-match）；`pathSet`（`internal/evidence/evidence.go:918`，调用点 L569，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/evidence`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`
- **需同步检查：** `internal/evidence/evidence.go`、`internal/tool/builtin/completestep.go`、`internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`

#### 待修改项

- [ ] 逐段核对 L570、L576、L581、L584、L588 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `PathsProvenInSession` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/evidence/evidence.go|PathsProvenInSession` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/evidence/evidence.go:568-595` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `PathsProvenInSession` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G47 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/evidence`。
- [ ] 运行相关包 vet：`go vet ./internal/evidence`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/evidence/evidence.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/evidence/evidence.go|PathsProvenInSession` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-239 · 圈复杂度 · `Aggregate`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-239 |
| Baseline key | `complexity\|internal/controlplane/consensus/consensus.go\|Aggregate` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/controlplane/consensus/consensus.go` |
| 代码行 | 24 |
| 函数/方法/包 | `Aggregate` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Aggregate` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 24-89 行；可见控制流热点：L25 `if len(signals) == 0 {`；L41 `if w <= 0 {`；L53 `if result.TotalWeight <= 0 {`；L62 `if w <= 0 {`；L70 `if score/result.TotalWeight > result.ConsensusScore {`；L76 `if w <= 0 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function Aggregate（internal/controlplane/consensus/consensus.go:24-89）
- **可信生产调用方（1）：** `Arbitrate`（`internal/controlplane/arbitration/consensus.go:13`，调用点 L14，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `actionValue`（`internal/controlplane/consensus/consensus.go:155`，调用点 L48/L65，confidence=0.90，exact-match，2 条边）；`adjustedWeights`（`internal/controlplane/consensus/consensus.go:91`，调用点 L30，confidence=0.90，exact-match）；`capDominantWeights`（`internal/controlplane/consensus/consensus.go:121`，调用点 L31，confidence=0.90，exact-match）；`ClampRate`（`internal/controlplane/control_graph/types.go:124`，调用点 L57，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/controlplane`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/controlplane/consensus/consensus_test.go`
- **需同步检查：** `internal/controlplane/consensus/consensus.go`、`internal/controlplane/arbitration/consensus.go`、`internal/controlplane/control_graph/types.go`、`internal/controlplane/consensus/consensus_test.go`

#### 待修改项

- [ ] 逐段核对 L25、L49、L60、L70、L76 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `Aggregate` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/controlplane/consensus/consensus.go|Aggregate` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/controlplane/consensus/consensus.go:24-89` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Aggregate` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G47 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/controlplane/consensus/consensus_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/controlplane/consensus`。
- [ ] 运行相关包 vet：`go vet ./internal/controlplane/consensus`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/controlplane/consensus/consensus.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/controlplane/consensus/consensus.go|Aggregate` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-240 · 圈复杂度 · `LearnControlGraph`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-240 |
| Baseline key | `complexity\|internal/controlplane/policy_nodes/weighting.go\|LearnControlGraph` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/controlplane/policy_nodes/weighting.go` |
| 代码行 | 36 |
| 函数/方法/包 | `LearnControlGraph` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`LearnControlGraph` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 36-51 行；可见控制流热点：L38 `for i, edge := range out.Edges {`；L39 `switch {`；L40 `case st.Oscillating && edge.From == "stability-controller" && edge.To == "exploration-controller":`；L42 `case len(st.SemanticShift) > 0 && edge.From == "semantic-drift-controller" && edge.To == "explorati…`；L44 `case st.Stable && edge.From == "exploration-controller" && edge.To == "mutation-controller":`；L46 `case st.Unstable && edge.From == "mutation-controller" && edge.To == "stability-controller":`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function LearnControlGraph（internal/controlplane/policy_nodes/weighting.go:36-51）
- **可信生产调用方（1）：** `DecideWithGraphAndHistory`（`internal/controlplane/controlplane.go:34`，调用点 L35，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `CloneGraph`（`internal/controlplane/control_graph/types.go:149`，调用点 L37，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/controlplane`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/controlplane/policy_nodes/weighting.go`、`internal/controlplane/controlplane.go`、`internal/controlplane/control_graph/types.go`

#### 待修改项

- [ ] 逐段核对 L38、L39、L42、L44、L46 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `LearnControlGraph` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/controlplane/policy_nodes/weighting.go|LearnControlGraph` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/controlplane/policy_nodes/weighting.go:36-51` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `LearnControlGraph` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G47 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 补充或确认覆盖当前职责、错误分支和边界条件的聚焦测试；不得仅依赖全量测试。
- [ ] 运行相关包测试：`go test ./internal/controlplane/policy_nodes`。
- [ ] 运行相关包 vet：`go vet ./internal/controlplane/policy_nodes`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/controlplane/policy_nodes/weighting.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/controlplane/policy_nodes/weighting.go|LearnControlGraph` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-255 · 圈复杂度 · `hasSuccessfulCompleteStepForTodo`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-255 |
| Baseline key | `complexity\|internal/evidence/evidence.go\|hasSuccessfulCompleteStepForTodo` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/evidence/evidence.go` |
| 代码行 | 796 |
| 函数/方法/包 | `hasSuccessfulCompleteStepForTodo` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`hasSuccessfulCompleteStepForTodo` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 796-818 行；可见控制流热点：L797 `for _, r := range receipts {`；L798 `if !r.Success || r.ToolName != "complete_step" || strings.TrimSpace(r.Step) == "" {`；L801 `if r.TodoStep != nil && r.TodoStep.Found {`；L805 `if sameTodoMatch(current[index-1], *r.TodoStep) {`；L808 `if !todoContentRelates(current[index-1], *r.TodoStep) {`；L813 `if match.Found && match.Index == index {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function hasSuccessfulCompleteStepForTodo（internal/evidence/evidence.go:796-818）
- **可信生产调用方（1）：** `Ledger::UnverifiedCompletedTodos`（`internal/evidence/evidence.go:416`，调用点 L450，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `sameTodoMatch`（`internal/evidence/evidence.go:831`，调用点 L805，confidence=0.90，exact-match）；`todoContentRelates`（`internal/evidence/evidence.go:839`，调用点 L808，confidence=0.90，exact-match）；`matchTodoStep`（`internal/evidence/evidence.go:848`，调用点 L812，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/evidence`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`
- **需同步检查：** `internal/evidence/evidence.go`、`internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`

#### 待修改项

- [ ] 逐段核对 L797、L801、L802、L808、L813 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `hasSuccessfulCompleteStepForTodo` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/evidence/evidence.go|hasSuccessfulCompleteStepForTodo` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/evidence/evidence.go:796-818` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `hasSuccessfulCompleteStepForTodo` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G47 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/evidence`。
- [ ] 运行相关包 vet：`go vet ./internal/evidence`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/evidence/evidence.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/evidence/evidence.go|hasSuccessfulCompleteStepForTodo` 下降或消失且无新增/恶化。
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
