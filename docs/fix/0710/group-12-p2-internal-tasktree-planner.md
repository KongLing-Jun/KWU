# G12 · internal/tasktree · planner 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G12 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/tasktree` |
| 涉及文件 | `internal/tasktree/planner.go`<br>`internal/tasktree/parallel.go`<br>`internal/tasktree/executor.go`<br>`internal/tasktree/scheduler.go`<br>`internal/tasktree/status.go`<br>`internal/tasktree/orchestrator.go` |
| 主要治理目标 | 围绕 `internal/tasktree/planner.go` 等治理对象降低复杂度/体量，统一处理 1 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | tasktree 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 低：仍需开始前复查开放 PR |
| 冲突依据 | 未发现静态热点或本地远端引用重叠 |
| 建议处理顺序 | 总序第 12 组；无同文件组依赖 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 返回值、错误语义和现有调用顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；并发、取消或事件顺序；会话、状态或恢复语义 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **56/100**，对象 `validateGeneratedNodes` 的分项为严重度 14.3、集中度 10.3、影响面 11.7、可执行性 20.0；最高超限 3.30x，文件内 4 条问题，CodeGraph 记录 1 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数，必须合并治理。

### 治理边界与组间关系

本组涉及 6 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-366` | `complexity\|internal/tasktree/planner.go\|validateGeneratedNodes` | 圈复杂度 | `internal/tasktree/planner.go` | `validateGeneratedNodes` | 205 | 33/10 | +23（3.30x） | P2 | 1 可信 + 0 待核 caller / 4 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `FN-049` | `func_lines\|internal/tasktree/planner.go\|validateGeneratedNodes` | 函数有效行数 | `internal/tasktree/planner.go` | `validateGeneratedNodes` | 205 | 97/80 | +17（1.21x） | P2 | 1 可信 + 0 待核 caller / 4 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-363` | `complexity\|internal/tasktree/parallel.go\|*Orchestrator.ExecuteBatch` | 圈复杂度 | `internal/tasktree/parallel.go` | `*Orchestrator.ExecuteBatch` | 70 | 18/10 | +8（1.80x） | P3 | 0 可信 + 1 待核 caller / 6 可信 + 10 待核 callee / 0 unresolved | 待修改 |
| `CX-364` | `complexity\|internal/tasktree/planner.go\|*LLMPlanner.PlanTaskTree` | 圈复杂度 | `internal/tasktree/planner.go` | `*LLMPlanner.PlanTaskTree` | 62 | 16/10 | +6（1.60x） | P3 | 0 可信 + 0 待核 caller / 3 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-361` | `complexity\|internal/tasktree/executor.go\|*Orchestrator.ExecuteOne` | 圈复杂度 | `internal/tasktree/executor.go` | `*Orchestrator.ExecuteOne` | 81 | 18/10 | +8（1.80x） | P3 | 0 可信 + 1 待核 caller / 0 可信 + 13 待核 callee / 0 unresolved | 待修改 |
| `CX-365` | `complexity\|internal/tasktree/planner.go\|NormalizeAndValidateGeneratedTree` | 圈复杂度 | `internal/tasktree/planner.go` | `NormalizeAndValidateGeneratedTree` | 160 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-368` | `complexity\|internal/tasktree/scheduler.go\|ReadyBatchNodeIDs` | 圈复杂度 | `internal/tasktree/scheduler.go` | `ReadyBatchNodeIDs` | 131 | 12/10 | +2（1.20x） | P3 | 3 可信 + 0 待核 caller / 0 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-369` | `complexity\|internal/tasktree/status.go\|CanTransition` | 圈复杂度 | `internal/tasktree/status.go` | `CanTransition` | 45 | 14/10 | +4（1.40x） | P3 | 2 可信 + 0 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-367` | `complexity\|internal/tasktree/scheduler.go\|*TaskTree.OrderedNodeIDs` | 圈复杂度 | `internal/tasktree/scheduler.go` | `*TaskTree.OrderedNodeIDs` | 9 | 12/10 | +2（1.20x） | P3 | 0 可信 + 3 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-362` | `complexity\|internal/tasktree/orchestrator.go\|*Orchestrator.Run` | 圈复杂度 | `internal/tasktree/orchestrator.go` | `*Orchestrator.Run` | 171 | 11/10 | +1（1.10x） | P3 | 0 可信 + 0 待核 caller / 0 可信 + 6 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-366 · 圈复杂度 · `validateGeneratedNodes`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-366 |
| Baseline key | `complexity\|internal/tasktree/planner.go\|validateGeneratedNodes` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 56） |
| 文件 | `internal/tasktree/planner.go` |
| 代码行 | 205 |
| 函数/方法/包 | `validateGeneratedNodes` |
| Baseline / 当前复测 / 阈值 | 33 / 33 / 10 |
| 超出 | +23，3.30x |
| 同一治理对象关联问题 | `FN-049` |

#### 明确的问题描述

`validateGeneratedNodes` 的圈复杂度当前为 **33**，超过限制 **10**（+23，3.30x）。定义范围为 205-306 行；可见控制流热点：L207 `for id, node := range tree.Nodes {`；L227 `if node.Title == "" {`；L248 `for _, rootID := range tree.RootIDs {`；L265 `if node.ParentID != "" {`；L282 `if child.ParentID != id {`；L302 `if err := validateDependencyAcyclic(tree); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（validateIDList、validateHierarchyAcyclic、validateHierarchyReachable、validateDependencyAcyclic）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function validateGeneratedNodes（internal/tasktree/planner.go:205-306）
- **可信生产调用方（1）：** `NormalizeAndValidateGeneratedTree`（`internal/tasktree/planner.go:160`，调用点 L199，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `validateIDList`（`internal/tasktree/planner.go:308`，调用点 L245/L274/L286，confidence=0.90，exact-match，3 条边）；`validateHierarchyAcyclic`（`internal/tasktree/planner.go:328`，调用点 L296，confidence=0.90，exact-match）；`validateHierarchyReachable`（`internal/tasktree/planner.go:357`，调用点 L299，confidence=0.90，exact-match）；`validateDependencyAcyclic`（`internal/tasktree/planner.go:380`，调用点 L302，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `ExecutorKind::Valid`（`internal/tasktree/result.go:27`，调用点 L233/L236，confidence=0.70，exact-match，2 条边，待源码复核）；`contains`（`internal/tasktree/planner.go:409`，调用点 L270，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `LLMPlanner::PlanTaskTree`（`internal/tasktree/planner.go:62`，调用点 L122，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/tasktree`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`、`internal/tasktree/parallel_test.go`、`internal/tasktree/planner_test.go`、`internal/tasktree/regression_checklist_test.go`、`internal/tasktree/resume_test.go`、`internal/tasktree/scheduler_test.go`、`internal/tasktree/serialize_test.go`、`internal/tasktree/state_test.go`
- **需同步检查：** `internal/tasktree/planner.go`、`internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`

#### 待修改项

- [ ] 逐段核对 L207、L233、L259、L278、L302 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `validateGeneratedNodes` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-366、FN-049 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tasktree/planner.go|validateGeneratedNodes` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tasktree/planner.go:205-306` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `validateGeneratedNodes` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-366`、`FN-049` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tasktree`。
- [ ] 运行相关包 vet：`go vet ./internal/tasktree`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tasktree/planner.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tasktree/planner.go|validateGeneratedNodes` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-049 · 函数有效行数 · `validateGeneratedNodes`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-049 |
| Baseline key | `func_lines\|internal/tasktree/planner.go\|validateGeneratedNodes` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 56） |
| 文件 | `internal/tasktree/planner.go` |
| 代码行 | 205 |
| 函数/方法/包 | `validateGeneratedNodes` |
| Baseline / 当前复测 / 阈值 | 97 / 97 / 80 |
| 超出 | +17，1.21x |
| 同一治理对象关联问题 | `CX-366` |

#### 明确的问题描述

`validateGeneratedNodes` 的有效代码行 **97**，超过限制 **80**（+17）。CodeGraph 定义物理范围为 205-306 行，并记录 6 个直接下游候选；候选职责分布为 配置/解析（validateIDList、validateHierarchyAcyclic、validateHierarchyReachable、validateDependencyAcyclic）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function validateGeneratedNodes（internal/tasktree/planner.go:205-306）
- **可信生产调用方（1）：** `NormalizeAndValidateGeneratedTree`（`internal/tasktree/planner.go:160`，调用点 L199，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `validateIDList`（`internal/tasktree/planner.go:308`，调用点 L245/L274/L286，confidence=0.90，exact-match，3 条边）；`validateHierarchyAcyclic`（`internal/tasktree/planner.go:328`，调用点 L296，confidence=0.90，exact-match）；`validateHierarchyReachable`（`internal/tasktree/planner.go:357`，调用点 L299，confidence=0.90，exact-match）；`validateDependencyAcyclic`（`internal/tasktree/planner.go:380`，调用点 L302，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `ExecutorKind::Valid`（`internal/tasktree/result.go:27`，调用点 L233/L236，confidence=0.70，exact-match，2 条边，待源码复核）；`contains`（`internal/tasktree/planner.go:409`，调用点 L270，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `LLMPlanner::PlanTaskTree`（`internal/tasktree/planner.go:62`，调用点 L122，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/tasktree`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`、`internal/tasktree/parallel_test.go`、`internal/tasktree/planner_test.go`、`internal/tasktree/regression_checklist_test.go`、`internal/tasktree/resume_test.go`、`internal/tasktree/scheduler_test.go`、`internal/tasktree/serialize_test.go`、`internal/tasktree/state_test.go`
- **需同步检查：** `internal/tasktree/planner.go`、`internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`

#### 待修改项

- [ ] 把 `validateGeneratedNodes` 的 97 个有效行按 配置/解析 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-366、FN-049 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/tasktree/planner.go|validateGeneratedNodes` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tasktree/planner.go:205-306` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `validateGeneratedNodes` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-366`、`FN-049` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tasktree`。
- [ ] 运行相关包 vet：`go vet ./internal/tasktree`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tasktree/planner.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/tasktree/planner.go|validateGeneratedNodes` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-363 · 圈复杂度 · `*Orchestrator.ExecuteBatch`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-363 |
| Baseline key | `complexity\|internal/tasktree/parallel.go\|*Orchestrator.ExecuteBatch` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 41） |
| 文件 | `internal/tasktree/parallel.go` |
| 代码行 | 70 |
| 函数/方法/包 | `*Orchestrator.ExecuteBatch` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Orchestrator.ExecuteBatch` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 70-135 行；可见控制流热点：L71 `if err := ctx.Err(); err != nil {`；L81 `if len(ids) == 0 {`；L93 `for _, id := range ids {`；L105 `if err := o.saveTaskTree(ctx, tree); err != nil {`；L116 `if err := tree.RecordResult(result.NodeID, nodeResult); err != nil {`；L131 `if batchHasFailure(results) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeBatchIDs、validateReadyBatchMembership、normalizeDispatchResult、Orchestrator::loadTaskTree）、状态与控制（normalizeDispatchResult、TaskTree::TransitionNode、Orchestrator::emitNodeStarted、Orchestrator::dispatchBatch）、持久化/I-O（validateReadyBatchMembership、Orchestrator::saveTaskTree、ArchiveSink::Err）、渲染/输出（Orchestrator::emitNodeStarted、Orchestrator::emitNodePhase、Orchestrator::emitNodeFinished）、任务/并发（Orchestrator::saveTaskTree、TaskTree::Node、TaskTree::TransitionNode、Orchestrator::loadTaskTree）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Orchestrator::ExecuteBatch（internal/tasktree/parallel.go:70-135）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Orchestrator::ExecuteReadyBatch`（`internal/tasktree/parallel.go:51`，调用点 L63，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（6）：** `normalizeBatchIDs`（`internal/tasktree/parallel.go:200`，调用点 L77，confidence=0.90，exact-match）；`validateReadyBatchMembership`（`internal/tasktree/parallel.go:217`，调用点 L89，confidence=0.90，exact-match）；`normalizeDispatchResult`（`internal/tasktree/parallel.go:234`，调用点 L115，confidence=0.90，exact-match）；`batchResultErr`（`internal/tasktree/parallel.go:251`，调用点 L122，confidence=0.90，exact-match）；`batchHasFailure`（`internal/tasktree/parallel.go:261`，调用点 L131，confidence=0.90，exact-match）；`cloneBatchNodeResults`（`internal/tasktree/parallel.go:270`，调用点 L132，confidence=0.90，exact-match）
- **待源码复核被调候选（10）：** `Orchestrator::saveTaskTree`（`internal/tasktree/executor.go:193`，调用点 L105/L125，confidence=0.70，instance-method，2 条边，待源码复核）；`TaskTree::Node`（`internal/tasktree/state.go:67`，调用点 L94/L119，confidence=0.70，instance-method，2 条边，待源码复核）；`TaskTree::TransitionNode`（`internal/tasktree/status.go:86`，调用点 L96/L100，confidence=0.70，instance-method，2 条边，待源码复核）；`ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L71，confidence=0.70，instance-method，待源码复核）；`Orchestrator::emitNodeStarted`（`internal/tasktree/events.go:27`，调用点 L109，confidence=0.70，instance-method，待源码复核）；`Orchestrator::loadTaskTree`（`internal/tasktree/executor.go:179`，调用点 L85，confidence=0.70，instance-method，待源码复核）；`Orchestrator::emitNodePhase`（`internal/tasktree/executor.go:201`，调用点 L110，confidence=0.70，instance-method，待源码复核）；`Orchestrator::emitNodeFinished`（`internal/tasktree/executor.go:209`，调用点 L129，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 7 个文件；`internal/tasktree`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/tasktree/parallel_test.go`
- **同目录测试候选：** `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`、`internal/tasktree/planner_test.go`、`internal/tasktree/regression_checklist_test.go`、`internal/tasktree/resume_test.go`、`internal/tasktree/scheduler_test.go`、`internal/tasktree/serialize_test.go`、`internal/tasktree/state_test.go`
- **需同步检查：** `internal/tasktree/parallel.go`、`internal/tasktree/parallel_test.go`、`internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`

#### 待修改项

- [ ] 逐段核对 L71、L86、L96、L114、L131 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Orchestrator.ExecuteBatch` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tasktree/parallel.go|*Orchestrator.ExecuteBatch` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tasktree/parallel.go:70-135` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Orchestrator.ExecuteBatch` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/tasktree/parallel_test.go`。
- [ ] 运行相关包测试：`go test ./internal/tasktree`。
- [ ] 运行相关包 race：`go test -race ./internal/tasktree`。
- [ ] 运行相关包 vet：`go vet ./internal/tasktree`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tasktree/parallel.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tasktree/parallel.go|*Orchestrator.ExecuteBatch` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-364 · 圈复杂度 · `*LLMPlanner.PlanTaskTree`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-364 |
| Baseline key | `complexity\|internal/tasktree/planner.go\|*LLMPlanner.PlanTaskTree` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 41） |
| 文件 | `internal/tasktree/planner.go` |
| 代码行 | 62 |
| 函数/方法/包 | `*LLMPlanner.PlanTaskTree` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*LLMPlanner.PlanTaskTree` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 62-123 行；可见控制流热点：L63 `if err := ctx.Err(); err != nil {`；L84 `if err != nil {`；L91 `case provider.ChunkText:`；L95 `case provider.ChunkToolCallStart, provider.ChunkToolCall:`；L102 `if chunk.Err != nil {`；L119 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（ParseTreeJSON、NormalizeAndValidateGeneratedTree）、状态与控制（ParseTreeJSON）、持久化/I-O（ArchiveSink::Err）、渲染/输出（RenderTaskTreePrompt）、任务/并发（RenderTaskTreePrompt）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method LLMPlanner::PlanTaskTree（internal/tasktree/planner.go:62-123）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `ParseTreeJSON`（`internal/tasktree/planner.go:127`，调用点 L118，confidence=0.90，exact-match）；`NormalizeAndValidateGeneratedTree`（`internal/tasktree/planner.go:160`，调用点 L122，confidence=0.90，exact-match）；`RenderTaskTreePrompt`（`internal/tasktree/planner_prompt.go:59`，调用点 L77，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L63/L110，confidence=0.70，instance-method，2 条边，待源码复核）；`Selector::now`（`internal/profile/selector.go:291`，调用点 L122，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/tasktree`、`internal/automation`、`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`、`internal/tasktree/parallel_test.go`、`internal/tasktree/planner_test.go`、`internal/tasktree/regression_checklist_test.go`、`internal/tasktree/resume_test.go`、`internal/tasktree/scheduler_test.go`、`internal/tasktree/serialize_test.go`、`internal/tasktree/state_test.go`
- **需同步检查：** `internal/tasktree/planner.go`、`internal/tasktree/planner_prompt.go`、`internal/automation/archive_sink.go`、`internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`

#### 待修改项

- [ ] 逐段核对 L63、L89、L93、L102、L119 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*LLMPlanner.PlanTaskTree` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tasktree/planner.go|*LLMPlanner.PlanTaskTree` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tasktree/planner.go:62-123` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*LLMPlanner.PlanTaskTree` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tasktree`。
- [ ] 运行相关包 vet：`go vet ./internal/tasktree`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tasktree/planner.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tasktree/planner.go|*LLMPlanner.PlanTaskTree` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-361 · 圈复杂度 · `*Orchestrator.ExecuteOne`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-361 |
| Baseline key | `complexity\|internal/tasktree/executor.go\|*Orchestrator.ExecuteOne` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/tasktree/executor.go` |
| 代码行 | 81 |
| 函数/方法/包 | `*Orchestrator.ExecuteOne` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Orchestrator.ExecuteOne` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 81-143 行；可见控制流热点：L82 `if err := ctx.Err(); err != nil {`；L94 `if err != nil {`；L106 `if node.Status == StatusPending {`；L111 `if err := tree.TransitionNode(nodeID, StatusRunning); err != nil {`；L128 `if dispatchErr != nil {`；L138 `if err := o.saveTaskTree(ctx, tree); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Orchestrator::loadTaskTree）、状态与控制（Orchestrator::finishNodeAfterDispatchError、TaskTree::TransitionNode、Orchestrator::emitNodeStarted、Orchestrator::dispatchNode）、持久化/I-O（Orchestrator::saveTaskTree、ArchiveSink::Err、TaskTree::NodeReadyToRun）、渲染/输出（Orchestrator::emitNodeStarted、Orchestrator::emitNodePhase、Orchestrator::emitNodeFinished）、任务/并发（Orchestrator::saveTaskTree、TaskTree::TransitionNode、TaskTree::BuildNodeContext、Orchestrator::loadTaskTree）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Orchestrator::ExecuteOne（internal/tasktree/executor.go:81-143）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Orchestrator::ExecuteNext`（`internal/tasktree/executor.go:64`，调用点 L73，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（13）：** `Orchestrator::finishNodeAfterDispatchError`（`internal/tasktree/executor.go:157`，调用点 L124/L129，confidence=0.70，instance-method，2 条边，待源码复核）；`Orchestrator::saveTaskTree`（`internal/tasktree/executor.go:193`，调用点 L115/L138，confidence=0.70，instance-method，2 条边，待源码复核）；`TaskTree::TransitionNode`（`internal/tasktree/status.go:86`，调用点 L107/L111，confidence=0.70，instance-method，2 条边，待源码复核）；`ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L82，confidence=0.70，instance-method，待源码复核）；`TaskTree::BuildNodeContext`（`internal/tasktree/context.go:34`，调用点 L121，confidence=0.70，instance-method，待源码复核）；`Orchestrator::emitNodeStarted`（`internal/tasktree/events.go:27`，调用点 L118，confidence=0.70，instance-method，待源码复核）；`Orchestrator::dispatchNode`（`internal/tasktree/executor.go:145`，调用点 L127，confidence=0.70，instance-method，待源码复核）；`Orchestrator::loadTaskTree`（`internal/tasktree/executor.go:179`，调用点 L93，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Orchestrator::ExecuteNext`（`internal/tasktree/executor.go:64`，调用点 L73，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 8 个文件；`internal/tasktree`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`
- **同目录测试候选：** `internal/tasktree/context_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`、`internal/tasktree/parallel_test.go`、`internal/tasktree/planner_test.go`、`internal/tasktree/regression_checklist_test.go`、`internal/tasktree/resume_test.go`、`internal/tasktree/scheduler_test.go`、`internal/tasktree/serialize_test.go`、`internal/tasktree/state_test.go`
- **需同步检查：** `internal/tasktree/executor.go`、`internal/tasktree/status.go`、`internal/automation/archive_sink.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/context_test.go`、`internal/tasktree/orchestrator_test.go`

#### 待修改项

- [ ] 逐段核对 L82、L98、L107、L128、L138 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Orchestrator.ExecuteOne` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tasktree/executor.go|*Orchestrator.ExecuteOne` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tasktree/executor.go:81-143` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Orchestrator.ExecuteOne` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`。
- [ ] 运行相关包测试：`go test ./internal/tasktree`。
- [ ] 运行相关包 vet：`go vet ./internal/tasktree`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tasktree/executor.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tasktree/executor.go|*Orchestrator.ExecuteOne` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-365 · 圈复杂度 · `NormalizeAndValidateGeneratedTree`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-365 |
| Baseline key | `complexity\|internal/tasktree/planner.go\|NormalizeAndValidateGeneratedTree` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/tasktree/planner.go` |
| 代码行 | 160 |
| 函数/方法/包 | `NormalizeAndValidateGeneratedTree` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`NormalizeAndValidateGeneratedTree` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 160-203 行；可见控制流热点：L162 `if tree == nil {`；L169 `if strings.TrimSpace(tree.Goal) == "" {`；L177 `if !tree.Status.Valid() {`；L186 `if len(tree.RootIDs) == 0 {`；L192 `if tree.CreatedAt.IsZero() {`；L199 `if err := validateGeneratedNodes(tree); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（validateGeneratedNodes）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function NormalizeAndValidateGeneratedTree（internal/tasktree/planner.go:160-203）
- **可信生产调用方（1）：** `LLMPlanner::PlanTaskTree`（`internal/tasktree/planner.go:62`，调用点 L122，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `validateGeneratedNodes`（`internal/tasktree/planner.go:205`，调用点 L199，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `ExecutorKind::Valid`（`internal/tasktree/result.go:27`，调用点 L177，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `LLMPlanner::PlanTaskTree`（`internal/tasktree/planner.go:62`，调用点 L122，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/tasktree`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/tasktree/planner_test.go`
- **同目录测试候选：** `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`、`internal/tasktree/parallel_test.go`、`internal/tasktree/regression_checklist_test.go`、`internal/tasktree/resume_test.go`、`internal/tasktree/scheduler_test.go`、`internal/tasktree/serialize_test.go`、`internal/tasktree/state_test.go`
- **需同步检查：** `internal/tasktree/planner.go`、`internal/tasktree/result.go`、`internal/tasktree/planner_test.go`、`internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`

#### 待修改项

- [ ] 逐段核对 L162、L174、L183、L189、L199 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `NormalizeAndValidateGeneratedTree` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tasktree/planner.go|NormalizeAndValidateGeneratedTree` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tasktree/planner.go:160-203` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `NormalizeAndValidateGeneratedTree` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/tasktree/planner_test.go`。
- [ ] 运行相关包测试：`go test ./internal/tasktree`。
- [ ] 运行相关包 vet：`go vet ./internal/tasktree`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tasktree/planner.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tasktree/planner.go|NormalizeAndValidateGeneratedTree` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-368 · 圈复杂度 · `ReadyBatchNodeIDs`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-368 |
| Baseline key | `complexity\|internal/tasktree/scheduler.go\|ReadyBatchNodeIDs` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/tasktree/scheduler.go` |
| 代码行 | 131 |
| 函数/方法/包 | `ReadyBatchNodeIDs` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ReadyBatchNodeIDs` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 131-171 行；可见控制流热点：L132 `if tree == nil {`；L142 `for _, id := range ids {`；L147 `if ok {`；L151 `if len(batch) > 0 {`；L156 `if !ok {`；L163 `if err != nil || len(batch) > 0 {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（TaskTree::NodeReadyToRun）、持久化/I-O（TaskTree::NodeReadyToRun）、任务/并发（TaskTree::OrderedNodeIDs、TaskTree::NodeReadyToRun、TaskTree::Node）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ReadyBatchNodeIDs（internal/tasktree/scheduler.go:131-171）
- **可信生产调用方（3）：** `Orchestrator::ExecuteReadyBatch`（`internal/tasktree/parallel.go:51`，调用点 L56，confidence=0.90，exact-match）；`validateReadyBatchMembership`（`internal/tasktree/parallel.go:217`，调用点 L218，confidence=0.90，exact-match）；`NextReadyNodeID`（`internal/tasktree/scheduler.go:174`，调用点 L175，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（3）：** `TaskTree::OrderedNodeIDs`（`internal/tasktree/scheduler.go:9`，调用点 L135，confidence=0.70，instance-method，待源码复核）；`TaskTree::NodeReadyToRun`（`internal/tasktree/scheduler.go:88`，调用点 L143，confidence=0.70，instance-method，待源码复核）；`TaskTree::Node`（`internal/tasktree/state.go:67`，调用点 L155，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Orchestrator::ExecuteNext`（`internal/tasktree/executor.go:64`，调用点 L69，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/tasktree`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/tasktree/scheduler_test.go`
- **同目录测试候选：** `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`、`internal/tasktree/parallel_test.go`、`internal/tasktree/planner_test.go`、`internal/tasktree/regression_checklist_test.go`、`internal/tasktree/resume_test.go`、`internal/tasktree/serialize_test.go`、`internal/tasktree/state_test.go`
- **需同步检查：** `internal/tasktree/scheduler.go`、`internal/tasktree/parallel.go`、`internal/tasktree/state.go`、`internal/tasktree/scheduler_test.go`、`internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`

#### 待修改项

- [ ] 逐段核对 L132、L142、L151、L156、L163 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `ReadyBatchNodeIDs` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tasktree/scheduler.go|ReadyBatchNodeIDs` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tasktree/scheduler.go:131-171` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ReadyBatchNodeIDs` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/tasktree/scheduler_test.go`。
- [ ] 运行相关包测试：`go test ./internal/tasktree`。
- [ ] 运行相关包 race：`go test -race ./internal/tasktree`。
- [ ] 运行相关包 vet：`go vet ./internal/tasktree`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tasktree/scheduler.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tasktree/scheduler.go|ReadyBatchNodeIDs` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-369 · 圈复杂度 · `CanTransition`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-369 |
| Baseline key | `complexity\|internal/tasktree/status.go\|CanTransition` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/tasktree/status.go` |
| 代码行 | 45 |
| 函数/方法/包 | `CanTransition` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`CanTransition` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 45-65 行；可见控制流热点：L46 `if !from.Valid() || !to.Valid() {`；L49 `if from == to {`；L52 `if from.Terminal() {`；L56 `case StatusPending:`；L58 `case StatusReady:`；L60 `case StatusRunning:`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function CanTransition（internal/tasktree/status.go:45-65）
- **可信生产调用方（2）：** `TaskNode::TransitionTo`（`internal/tasktree/status.go:68`，调用点 L78，confidence=0.90，exact-match）；`TaskTree::TransitionNode`（`internal/tasktree/status.go:86`，调用点 L91，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `Status::Terminal`（`internal/tasktree/status.go:29`，调用点 L52，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tasktree`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`、`internal/tasktree/parallel_test.go`、`internal/tasktree/planner_test.go`、`internal/tasktree/regression_checklist_test.go`、`internal/tasktree/resume_test.go`、`internal/tasktree/scheduler_test.go`、`internal/tasktree/serialize_test.go`、`internal/tasktree/state_test.go`
- **需同步检查：** `internal/tasktree/status.go`、`internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`

#### 待修改项

- [ ] 逐段核对 L46、L52、L55、L58、L60 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `CanTransition` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tasktree/status.go|CanTransition` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tasktree/status.go:45-65` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `CanTransition` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tasktree`。
- [ ] 运行相关包 vet：`go vet ./internal/tasktree`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tasktree/status.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tasktree/status.go|CanTransition` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-367 · 圈复杂度 · `*TaskTree.OrderedNodeIDs`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-367 |
| Baseline key | `complexity\|internal/tasktree/scheduler.go\|*TaskTree.OrderedNodeIDs` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/tasktree/scheduler.go` |
| 代码行 | 9 |
| 函数/方法/包 | `*TaskTree.OrderedNodeIDs` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*TaskTree.OrderedNodeIDs` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 9-56 行；可见控制流热点：L10 `if t == nil {`；L23 `if !ok {`；L29 `if visited[id] {`；L36 `if err := walk(childID); err != nil {`；L46 `if err := walk(rootID); err != nil {`；L51 `if !visited[id] {`。CodeGraph 记录的下游职责候选主要涉及 任务/并发（TaskTree::Node）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method TaskTree::OrderedNodeIDs（internal/tasktree/scheduler.go:9-56）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（3）：** `orderedNodeIDsForEvent`（`internal/tasktree/events.go:117`，调用点 L118，confidence=0.70，instance-method，待源码复核）；`ReadyNodeIDs`（`internal/tasktree/scheduler.go:107`，调用点 L111，confidence=0.70，instance-method，待源码复核）；`ReadyBatchNodeIDs`（`internal/tasktree/scheduler.go:131`，调用点 L135，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `TaskTree::Node`（`internal/tasktree/state.go:67`，调用点 L22，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `ReadyNodeIDs`（`internal/tasktree/scheduler.go:107`，调用点 L111，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/tasktree`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`、`internal/tasktree/parallel_test.go`、`internal/tasktree/planner_test.go`、`internal/tasktree/regression_checklist_test.go`、`internal/tasktree/resume_test.go`、`internal/tasktree/scheduler_test.go`、`internal/tasktree/serialize_test.go`、`internal/tasktree/state_test.go`
- **需同步检查：** `internal/tasktree/scheduler.go`、`internal/tasktree/events.go`、`internal/tasktree/state.go`、`internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`

#### 待修改项

- [ ] 逐段核对 L10、L26、L35、L46、L51 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*TaskTree.OrderedNodeIDs` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tasktree/scheduler.go|*TaskTree.OrderedNodeIDs` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tasktree/scheduler.go:9-56` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*TaskTree.OrderedNodeIDs` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tasktree`。
- [ ] 运行相关包 vet：`go vet ./internal/tasktree`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tasktree/scheduler.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tasktree/scheduler.go|*TaskTree.OrderedNodeIDs` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-362 · 圈复杂度 · `*Orchestrator.Run`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-362 |
| Baseline key | `complexity\|internal/tasktree/orchestrator.go\|*Orchestrator.Run` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/tasktree/orchestrator.go` |
| 代码行 | 171 |
| 函数/方法/包 | `*Orchestrator.Run` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Orchestrator.Run` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 171-233 行；可见控制流热点：L172 `if err := ctx.Err(); err != nil {`；L182 `if err != nil {`；L185 `if errors.Is(err, ErrNotImplemented) {`；L197 `if tree == nil {`；L218 `if err != nil {`；L229 `if _, err := o.Summarize(ctx, decisions...); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（Orchestrator::executePlannedTree、StateStore::SaveSnapshot）、持久化/I-O（ArchiveSink::Err、StateStore::SaveSnapshot）、渲染/输出（Orchestrator::emitTreeCreated）、任务/并发（PlannerFunc::PlanTaskTree）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Orchestrator::Run（internal/tasktree/orchestrator.go:171-233）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（6）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L172/L183/L219，confidence=0.70，instance-method，3 条边，待源码复核）；`Orchestrator::emitTreeCreated`（`internal/tasktree/events.go:10`，调用点 L214，confidence=0.70，instance-method，待源码复核）；`PlannerFunc::PlanTaskTree`（`internal/tasktree/orchestrator.go:32`，调用点 L181，confidence=0.70，exact-match，待源码复核）；`Orchestrator::executePlannedTree`（`internal/tasktree/orchestrator.go:235`，调用点 L217，confidence=0.70，instance-method，待源码复核）；`StateStore::SaveSnapshot`（`internal/tasktree/store.go:12`，调用点 L206，confidence=0.70，exact-match，待源码复核）；`Orchestrator::Summarize`（`internal/tasktree/summary.go:169`，调用点 L229，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/tasktree`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`、`internal/tasktree/parallel_test.go`、`internal/tasktree/planner_test.go`、`internal/tasktree/regression_checklist_test.go`、`internal/tasktree/resume_test.go`、`internal/tasktree/scheduler_test.go`、`internal/tasktree/serialize_test.go`、`internal/tasktree/state_test.go`
- **需同步检查：** `internal/tasktree/orchestrator.go`、`internal/automation/archive_sink.go`、`internal/tasktree/events.go`、`internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`

#### 待修改项

- [ ] 逐段核对 L172、L182、L197、L218、L229 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Orchestrator.Run` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tasktree/orchestrator.go|*Orchestrator.Run` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tasktree/orchestrator.go:171-233` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Orchestrator.Run` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tasktree/context_test.go`、`internal/tasktree/dispatch_test.go`、`internal/tasktree/executor_test.go`、`internal/tasktree/orchestrator_test.go`、`internal/tasktree/outcome_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tasktree`。
- [ ] 运行相关包 vet：`go vet ./internal/tasktree`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tasktree/orchestrator.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tasktree/orchestrator.go|*Orchestrator.Run` 下降或消失且无新增/恶化。
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
