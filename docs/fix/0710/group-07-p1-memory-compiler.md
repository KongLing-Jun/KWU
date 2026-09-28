# G07 · Memory Compiler 运行时图与策略

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G07 |
| 问题数量 | 10 |
| 整体优先级 | P1 |
| 涉及模块 | `internal/memorycompiler` |
| 涉及文件 | `internal/memorycompiler/runtime.go` |
| 主要治理目标 | 围绕 `internal/memorycompiler/runtime.go` 等治理对象降低复杂度/体量，统一处理 1 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | memorycompiler 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 15 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 7 组；先于 G19 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **61/100**，对象 `internal/memorycompiler/runtime.go` 的分项为严重度 21.3、集中度 9.1、影响面 24.2、可执行性 6.0；最高超限 3.35x，文件内 15 条问题，CodeGraph 记录 7 个可信生产调用方、2 个待核候选。

### 治理边界与组间关系

本组问题集中在同一文件，建议由单一负责人统一处理。没有同文件前置组。后续同文件组：G19，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `FL-016` | `file_lines\|internal/memorycompiler/runtime.go\|` | 文件有效行数 | `internal/memorycompiler/runtime.go` | `internal/memorycompiler/runtime.go` | 1 | 2677/800 | +1877（3.35x） | P1 | 7 可信 + 2 待核 caller / 9 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-305` | `complexity\|internal/memorycompiler/runtime.go\|buildIRWithPolicy` | 圈复杂度 | `internal/memorycompiler/runtime.go` | `buildIRWithPolicy` | 544 | 32/10 | +22（3.20x） | P2 | 2 可信 + 0 待核 caller / 14 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `FN-042` | `func_lines\|internal/memorycompiler/runtime.go\|buildIRWithPolicy` | 函数有效行数 | `internal/memorycompiler/runtime.go` | `buildIRWithPolicy` | 544 | 94/80 | +14（1.18x） | P2 | 2 可信 + 0 待核 caller / 14 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-307` | `complexity\|internal/memorycompiler/runtime.go\|canonicalizeIR` | 圈复杂度 | `internal/memorycompiler/runtime.go` | `canonicalizeIR` | 789 | 12/10 | +2（1.20x） | P3 | 4 可信 + 0 待核 caller / 9 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-308` | `complexity\|internal/memorycompiler/runtime.go\|usableSubgraphNodes` | 圈复杂度 | `internal/memorycompiler/runtime.go` | `usableSubgraphNodes` | 1039 | 17/10 | +7（1.70x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `FN-043` | `func_lines\|internal/memorycompiler/runtime.go\|updateGraph` | 函数有效行数 | `internal/memorycompiler/runtime.go` | `updateGraph` | 2440 | 99/80 | +19（1.24x） | P3 | 1 可信 + 0 待核 caller / 10 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-310` | `complexity\|internal/memorycompiler/runtime.go\|controlPlaneSystemState` | 圈复杂度 | `internal/memorycompiler/runtime.go` | `controlPlaneSystemState` | 1354 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 6 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-312` | `complexity\|internal/memorycompiler/runtime.go\|analyzeTrace` | 圈复杂度 | `internal/memorycompiler/runtime.go` | `analyzeTrace` | 1960 | 16/10 | +6（1.60x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-311` | `complexity\|internal/memorycompiler/runtime.go\|validateIRExecution` | 圈复杂度 | `internal/memorycompiler/runtime.go` | `validateIRExecution` | 1636 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 6 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-315` | `complexity\|internal/memorycompiler/runtime.go\|classifyStrategy` | 圈复杂度 | `internal/memorycompiler/runtime.go` | `classifyStrategy` | 2796 | 12/10 | +2（1.20x） | P3 | 4 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### FL-016 · 文件有效行数 · `internal/memorycompiler/runtime.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-016 |
| Baseline key | `file_lines\|internal/memorycompiler/runtime.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 61） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/memorycompiler/runtime.go` |
| Baseline / 当前复测 / 阈值 | 2677 / 2677 / 800 |
| 超出 | +1877，3.35x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/memorycompiler/runtime.go` 当前有效代码行 **2677**，超过限制 **800**（+1877）。CodeGraph 在文件中索引到的主要大符号包括 updateGraph@2440-2540、buildIRWithPolicy@544-639、Runtime::StartTurn@431-497、canonicalizeIR@789-847、memoryCitationsForIR@726-780、usableSubgraphNodes@1039-1091；文件同时承载的调用影响覆盖 9 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 129 个函数/方法节点。
- **可信生产调用方（7）：** `crossGraphAlignment`（`internal/memorycompiler/compression.go:519`，调用点 L552/L553/L554，confidence=0.90，exact-match，6 条边）；`compressExecutionTrace`（`internal/memorycompiler/compression.go:412`，调用点 L420/L424，confidence=0.90，exact-match，4 条边）；`compressControlGraph`（`internal/memorycompiler/compression.go:440`，调用点 L451/L452，confidence=0.90，exact-match，4 条边）；`layerCollapseSuggestions`（`internal/memorycompiler/compression.go:1465`，调用点 L1482/L1470，confidence=0.90，exact-match，4 条边）；`compressCausalEdges`（`internal/memorycompiler/compression.go:287`，调用点 L310，confidence=0.90，exact-match，2 条边）；`compressMemoryGraph`（`internal/memorycompiler/compression.go:456`，调用点 L504，confidence=0.90，exact-match，2 条边）
- **待源码复核调用方候选（2）：** `Controller::ClearSession`（`internal/control/controller.go:1977`，调用点 L1997/L2000/L2033，confidence=0.70，instance-method，3 条边，待源码复核）；`Agent::Run`（`internal/agent/agent.go:675`，调用点 L692/L696，confidence=0.80，instance-method，2 条边，待源码复核）
- **可信被调用函数（9）：** `cloneCompressionReport`（`internal/memorycompiler/compression.go:626`，调用点 L1937/L1908，confidence=0.90，exact-match，2 条边）；`Action`（`internal/controlplane/control_graph/types.go:18`，调用点 L1329，confidence=0.90，import）；`DecideWithHistory`（`internal/controlplane/controlplane.go:30`，调用点 L1275，confidence=0.90，import）；`AtomicWriteFile`（`internal/fileutil/atomicwrite.go:20`，调用点 L2916，confidence=0.90，import）；`Encode`（`internal/fileutil/encoding/encoding.go:166`，调用点 L2904，confidence=0.90，exact-match）；`retainMemoryNodes`（`internal/memorycompiler/compression.go:577`，调用点 L2534，confidence=0.90，exact-match）；`retainMemoryEdges`（`internal/memorycompiler/compression.go:610`，调用点 L2535，confidence=0.90，exact-match）；`applyCausalCompression`（`internal/memorycompiler/compression.go:236`，调用点 L1866，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L486，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 9 个文件；`internal/memorycompiler`、`internal/control`、`internal/agent`、`internal/automation`、`internal/controlplane`、`internal/fileutil`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/memorycompiler/runtime_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/agent/loop_e2e_test.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`desktop/app_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression.go`、`internal/automation/archive_sink.go`、`internal/controlplane/control_graph/types.go`、`internal/controlplane/controlplane.go`、`internal/memorycompiler/runtime_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/agent/loop_e2e_test.go`、`internal/memorycompiler/compression_test.go`

#### 待修改项

- [ ] 先冻结 `internal/memorycompiler/runtime.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `updateGraph`、`buildIRWithPolicy`、`Runtime::StartTurn`、`canonicalizeIR`、`memoryCitationsForIR` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-305、CX-306、CX-307、CX-308、CX-309、CX-310、CX-311、CX-312、CX-313、CX-314、CX-315、FN-042、FN-043、ND-014。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/memorycompiler/runtime.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/memorycompiler/runtime.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G19 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/memorycompiler/runtime_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/agent/loop_e2e_test.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 race：`go test -race ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/memorycompiler/runtime.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-305 · 圈复杂度 · `buildIRWithPolicy`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-305 |
| Baseline key | `complexity\|internal/memorycompiler/runtime.go\|buildIRWithPolicy` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 56） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 544 |
| 函数/方法/包 | `buildIRWithPolicy` |
| Baseline / 当前复测 / 阈值 | 32 / 32 / 10 |
| 超出 | +22，3.20x |
| 同一治理对象关联问题 | `FN-042` |

#### 明确的问题描述

`buildIRWithPolicy` 的圈复杂度当前为 **32**，超过限制 **10**（+22，3.20x）。定义范围为 544-639 行；可见控制流热点：L557 `if strategyPick.Mode == "explore" {`；L571 `if count >= 2 {`；L592 `for _, m := range st.Mutations {`；L599 `case "increase_weight", "add_constraint":`；L619 `if ir.StrategySelection != nil && ir.StrategySelection.Selected != "" {`；L630 `if len(ir.ExecutionSteps) == 0 {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（controlPolicyForState、applyDriftControl）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function buildIRWithPolicy（internal/memorycompiler/runtime.go:544-639）
- **可信生产调用方（2）：** `Runtime::StartTurn`（`internal/memorycompiler/runtime.go:431`，调用点 L443，confidence=0.90，exact-match）；`buildIR`（`internal/memorycompiler/runtime.go:539`，调用点 L540，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（14）：** `appendConstraint`（`internal/memorycompiler/runtime.go:1001`，调用点 L562/L578/L598/L600，confidence=0.90，exact-match，5 条边）；`strategyPlan`（`internal/memorycompiler/runtime.go:1151`，调用点 L620/L625，confidence=0.90，exact-match，2 条边）；`canonicalizeIR`（`internal/memorycompiler/runtime.go:789`，调用点 L638，confidence=0.90，exact-match）；`usableSubgraphNodes`（`internal/memorycompiler/runtime.go:1039`，调用点 L576，confidence=0.90，exact-match）；`sortedNoisyRefs`（`internal/memorycompiler/runtime.go:1107`，调用点 L569，confidence=0.90，exact-match）；`influenceForNode`（`internal/memorycompiler/runtime.go:1121`，调用点 L585，confidence=0.90，exact-match）；`rankStrategies`（`internal/memorycompiler/runtime.go:1166`，调用点 L555，confidence=0.90，exact-match）；`selectStrategy`（`internal/memorycompiler/runtime.go:1181`，调用点 L556，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Strategy::Samples`（`internal/memorycompiler/runtime.go:372`，调用点 L607/L609，confidence=0.70，instance-method，2 条边，待源码复核）；`Strategy::SuccessRate`（`internal/memorycompiler/runtime.go:374`，调用点 L607，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L692，confidence=0.70，instance-method，待源码复核）；`buildIR`（`internal/memorycompiler/runtime.go:539`，调用点 L540，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `buildIRWithPolicy` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L557、L576、L596、L615、L630 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `buildIRWithPolicy` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-305、FN-042 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/runtime.go|buildIRWithPolicy` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:544-639` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `buildIRWithPolicy` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G19 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-305`、`FN-042` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/runtime.go|buildIRWithPolicy` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-042 · 函数有效行数 · `buildIRWithPolicy`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-042 |
| Baseline key | `func_lines\|internal/memorycompiler/runtime.go\|buildIRWithPolicy` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 56） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 544 |
| 函数/方法/包 | `buildIRWithPolicy` |
| Baseline / 当前复测 / 阈值 | 94 / 94 / 80 |
| 超出 | +14，1.18x |
| 同一治理对象关联问题 | `CX-305` |

#### 明确的问题描述

`buildIRWithPolicy` 的有效代码行 **94**，超过限制 **80**（+14）。CodeGraph 定义物理范围为 544-639 行，并记录 16 个直接下游候选；候选职责分布为 状态与控制（controlPolicyForState、applyDriftControl）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function buildIRWithPolicy（internal/memorycompiler/runtime.go:544-639）
- **可信生产调用方（2）：** `Runtime::StartTurn`（`internal/memorycompiler/runtime.go:431`，调用点 L443，confidence=0.90，exact-match）；`buildIR`（`internal/memorycompiler/runtime.go:539`，调用点 L540，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（14）：** `appendConstraint`（`internal/memorycompiler/runtime.go:1001`，调用点 L562/L578/L598/L600，confidence=0.90，exact-match，5 条边）；`strategyPlan`（`internal/memorycompiler/runtime.go:1151`，调用点 L620/L625，confidence=0.90，exact-match，2 条边）；`canonicalizeIR`（`internal/memorycompiler/runtime.go:789`，调用点 L638，confidence=0.90，exact-match）；`usableSubgraphNodes`（`internal/memorycompiler/runtime.go:1039`，调用点 L576，confidence=0.90，exact-match）；`sortedNoisyRefs`（`internal/memorycompiler/runtime.go:1107`，调用点 L569，confidence=0.90，exact-match）；`influenceForNode`（`internal/memorycompiler/runtime.go:1121`，调用点 L585，confidence=0.90，exact-match）；`rankStrategies`（`internal/memorycompiler/runtime.go:1166`，调用点 L555，confidence=0.90，exact-match）；`selectStrategy`（`internal/memorycompiler/runtime.go:1181`，调用点 L556，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Strategy::Samples`（`internal/memorycompiler/runtime.go:372`，调用点 L607/L609，confidence=0.70，instance-method，2 条边，待源码复核）；`Strategy::SuccessRate`（`internal/memorycompiler/runtime.go:374`，调用点 L607，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L692，confidence=0.70，instance-method，待源码复核）；`buildIR`（`internal/memorycompiler/runtime.go:539`，调用点 L540，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `buildIRWithPolicy` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `buildIRWithPolicy` 的 94 个有效行按 状态与控制 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-305、FN-042 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/memorycompiler/runtime.go|buildIRWithPolicy` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:544-639` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `buildIRWithPolicy` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G19 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-305`、`FN-042` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/memorycompiler/runtime.go|buildIRWithPolicy` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-307 · 圈复杂度 · `canonicalizeIR`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-307 |
| Baseline key | `complexity\|internal/memorycompiler/runtime.go\|canonicalizeIR` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 789 |
| 函数/方法/包 | `canonicalizeIR` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`canonicalizeIR` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 789-847 行；可见控制流热点：L791 `if ir.Version == "" {`；L805 `if ir.StrategySelection == nil {`；L821 `if ir.StrategySelection.Mode == "" {`；L831 `if ir.Constraints == nil {`；L837 `if ir.MemoryReferences == nil {`；L843 `if ir.RiskNotes == nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function canonicalizeIR（internal/memorycompiler/runtime.go:789-847）
- **可信生产调用方（4）：** `buildIRWithPolicy`（`internal/memorycompiler/runtime.go:544`，调用点 L638，confidence=0.90，exact-match）；`compileExecutionContract`（`internal/memorycompiler/runtime.go:668`，调用点 L676，confidence=0.90，exact-match）；`memoryCitationsForIR`（`internal/memorycompiler/runtime.go:726`，调用点 L727，confidence=0.90，exact-match）；`validateIRExecution`（`internal/memorycompiler/runtime.go:1636`，调用点 L1637，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（9）：** `canonicalConstraints`（`internal/memorycompiler/runtime.go:849`，调用点 L800，confidence=0.90，exact-match）；`canonicalStrategyRefs`（`internal/memorycompiler/runtime.go:885`，调用点 L801，confidence=0.90，exact-match）；`canonicalRejectedStrategies`（`internal/memorycompiler/runtime.go:906`，调用点 L829，confidence=0.90，exact-match）；`canonicalMemoryRefs`（`internal/memorycompiler/runtime.go:926`，调用点 L802，confidence=0.90，exact-match）；`canonicalSteps`（`internal/memorycompiler/runtime.go:950`，调用点 L803，confidence=0.90，exact-match）；`canonicalStrings`（`internal/memorycompiler/runtime.go:963`，调用点 L804，confidence=0.90，exact-match）；`roundScore`（`internal/memorycompiler/runtime.go:994`，调用点 L819，confidence=0.90，exact-match）；`clampExplorationRatePercent`（`internal/memorycompiler/runtime.go:1260`，调用点 L828，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `buildIR`（`internal/memorycompiler/runtime.go:539`，调用点 L540，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `canonicalizeIR` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L791、L815、L825、L837、L843 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `canonicalizeIR` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/runtime.go|canonicalizeIR` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:789-847` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `canonicalizeIR` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G19 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/runtime.go|canonicalizeIR` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-308 · 圈复杂度 · `usableSubgraphNodes`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-308 |
| Baseline key | `complexity\|internal/memorycompiler/runtime.go\|usableSubgraphNodes` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 1039 |
| 函数/方法/包 | `usableSubgraphNodes` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`usableSubgraphNodes` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 1039-1091 行；可见控制流热点：L1041 `if len(usable) == 0 {`；L1053 `if len(frontier) >= 5 {`；L1061 `if !traversableRelation(edge.Relation) {`；L1066 `case edge.From == current:`；L1074 `if _, ok := byID[next]; !ok {`；L1086 `if selected[node.ID] {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function usableSubgraphNodes（internal/memorycompiler/runtime.go:1039-1091）
- **可信生产调用方（1）：** `buildIRWithPolicy`（`internal/memorycompiler/runtime.go:544`，调用点 L576，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `usableNodes`（`internal/memorycompiler/runtime.go:1015`，调用点 L1040，confidence=0.90，exact-match）；`traversableRelation`（`internal/memorycompiler/runtime.go:1093`，调用点 L1061，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `buildIR`（`internal/memorycompiler/runtime.go:539`，调用点 L540，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `usableSubgraphNodes` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1041、L1057、L1065、L1074、L1086 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `usableSubgraphNodes` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/runtime.go|usableSubgraphNodes` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:1039-1091` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `usableSubgraphNodes` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G19 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/runtime.go|usableSubgraphNodes` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-043 · 函数有效行数 · `updateGraph`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-043 |
| Baseline key | `func_lines\|internal/memorycompiler/runtime.go\|updateGraph` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 2440 |
| 函数/方法/包 | `updateGraph` |
| Baseline / 当前复测 / 阈值 | 99 / 99 / 80 |
| 超出 | +19，1.24x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`updateGraph` 的有效代码行 **99**，超过限制 **80**（+19）。CodeGraph 定义物理范围为 2440-2540 行，并记录 12 个直接下游候选；候选职责分布为 CodeGraph 未提供足够的下游调用分类。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function updateGraph（internal/memorycompiler/runtime.go:2440-2540）
- **可信生产调用方（1）：** `Runtime::writeTraceAndLearn`（`internal/memorycompiler/runtime.go:1829`，调用点 L1851，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（10）：** `upsertNode`（`internal/memorycompiler/runtime.go:2591`，调用点 L2451/L2461/L2483/L2508，confidence=0.90，exact-match，5 条边）；`appendEdge`（`internal/memorycompiler/runtime.go:2617`，调用点 L2470/L2493/L2504/L2518，confidence=0.90，exact-match，5 条边）；`confidenceForOutcome`（`internal/memorycompiler/runtime.go:2629`，调用点 L2447/L2466，confidence=0.90，exact-match，2 条边）；`retainMemoryNodes`（`internal/memorycompiler/compression.go:577`，调用点 L2534，confidence=0.90，exact-match）；`retainMemoryEdges`（`internal/memorycompiler/compression.go:610`，调用点 L2535，confidence=0.90，exact-match）；`rejectedOptions`（`internal/memorycompiler/runtime.go:2542`，调用点 L2456，confidence=0.90，exact-match）；`graphRelation`（`internal/memorycompiler/runtime.go:2551`，调用点 L2496，confidence=0.90，exact-match）；`appendDecision`（`internal/memorycompiler/runtime.go:2608`，调用点 L2460，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `firstLine`（`internal/memorycompiler/runtime.go:2864`，调用点 L2480/L2481，confidence=0.70，exact-match，2 条边，待源码复核）；`firstNonEmpty`（`internal/memorycompiler/runtime.go:2662`，调用点 L2455，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `updateGraph` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `updateGraph` 的 99 个有效行按 输入、处理、输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/memorycompiler/runtime.go|updateGraph` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:2440-2540` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `updateGraph` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G19 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/memorycompiler/runtime.go|updateGraph` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-310 · 圈复杂度 · `controlPlaneSystemState`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-310 |
| Baseline key | `complexity\|internal/memorycompiler/runtime.go\|controlPlaneSystemState` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 1354 |
| 函数/方法/包 | `controlPlaneSystemState` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`controlPlaneSystemState` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 1354-1391 行；可见控制流热点：L1363 `for _, learning := range recent {`；L1367 `if len(learning.BadStrategies) > 0 {`；L1372 `for _, finding := range learning.CausalFindings {`；L1377 `if strings.Contains(lower, "semantic drift") {`；L1380 `if strings.Contains(lower, "memory ") && strings.Contains(lower, "failed outcome") {`；L1386 `if mutation.Applied && mutation.Status != "accepted" && mutation.Status != "rejected" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function controlPlaneSystemState（internal/memorycompiler/runtime.go:1354-1391）
- **可信生产调用方（1）：** `controlPolicyForState`（`internal/memorycompiler/runtime.go:1274`，调用点 L1275，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `semanticShiftSignals`（`internal/memorycompiler/runtime.go:1406`，调用点 L1361，confidence=0.90，exact-match）；`equilibriumUnstable`（`internal/memorycompiler/runtime.go:1438`，调用点 L1358，confidence=0.90，exact-match）；`equilibriumOscillating`（`internal/memorycompiler/runtime.go:1450`，调用点 L1359，confidence=0.90，exact-match）；`equilibriumStable`（`internal/memorycompiler/runtime.go:1481`，调用点 L1357，confidence=0.90，exact-match）；`recentLearnings`（`internal/memorycompiler/runtime.go:1497`，调用点 L1355，confidence=0.90，exact-match）；`hasDrift`（`internal/memorycompiler/runtime.go:2387`，调用点 L1360，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `equilibriumExplorationRatePercent`（`internal/memorycompiler/runtime.go:1270`，调用点 L1271，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `controlPlaneSystemState` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1363、L1367、L1374、L1380、L1386 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `controlPlaneSystemState` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/runtime.go|controlPlaneSystemState` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:1354-1391` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `controlPlaneSystemState` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G19 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/runtime.go|controlPlaneSystemState` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-312 · 圈复杂度 · `analyzeTrace`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-312 |
| Baseline key | `complexity\|internal/memorycompiler/runtime.go\|analyzeTrace` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 38） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 1960 |
| 函数/方法/包 | `analyzeTrace` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`analyzeTrace` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 1960-2010 行；可见控制流热点：L1963 `for _, rec := range tr.ToolResults {`；L1968 `for sig, n := range errorCounts {`；L1981 `for _, memoryID := range tr.MemoryUsed {`；L1987 `for _, memoryID := range tr.MemoryUsed {`；L2000 `for _, finding := range softDrift {`；L2006 `if tr.Cost.EstimatedIROverheadTokens > 800 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function analyzeTrace（internal/memorycompiler/runtime.go:1960-2010）
- **可信生产调用方（1）：** `Runtime::writeTraceAndLearn`（`internal/memorycompiler/runtime.go:1829`，调用点 L1846，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `dedupeLearning`（`internal/memorycompiler/runtime.go:2408`，调用点 L2009，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `firstLine`（`internal/memorycompiler/runtime.go:2864`，调用点 L1975/L1976，confidence=0.70，exact-match，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/memorycompiler/runtime_test.go`
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/runtime_test.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`

#### 待修改项

- [ ] 先冻结 `analyzeTrace` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1963、L1969、L1985、L1996、L2006 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `analyzeTrace` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/runtime.go|analyzeTrace` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:1960-2010` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `analyzeTrace` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G19 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/memorycompiler/runtime_test.go`。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/runtime.go|analyzeTrace` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-311 · 圈复杂度 · `validateIRExecution`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-311 |
| Baseline key | `complexity\|internal/memorycompiler/runtime.go\|validateIRExecution` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 1636 |
| 函数/方法/包 | `validateIRExecution` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`validateIRExecution` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 1636-1672 行；可见控制流热点：L1641 `if finding == "" {`；L1649 `if finding == "" {`；L1654 `if selected := selectedStrategy(ir); selected != "" && selected != "general" {`；L1659 `if !sameStepIDs(ir.ExecutionSteps, tr.Steps) {`；L1662 `if !sameStringSet(memoryRefIDs(ir.MemoryReferences), tr.MemoryUsed) {`；L1665 `if len(ir.ExecutionSteps) > 0 && tr.Cost.ToolCalls > len(ir.ExecutionSteps)+3 && tr.Cost.ToolCalls …`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（sameStringSet）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function validateIRExecution（internal/memorycompiler/runtime.go:1636-1672）
- **可信生产调用方（1）：** `Turn::Finish`（`internal/memorycompiler/runtime.go:1728`，调用点 L1738，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `canonicalStrings`（`internal/memorycompiler/runtime.go:963`，调用点 L1668/L1669/L1670，confidence=0.90，exact-match，3 条边）；`selectedStrategy`（`internal/memorycompiler/runtime.go:782`，调用点 L1654，confidence=0.90，exact-match）；`canonicalizeIR`（`internal/memorycompiler/runtime.go:789`，调用点 L1637，confidence=0.90，exact-match）；`memoryRefIDs`（`internal/memorycompiler/runtime.go:1592`，调用点 L1662，confidence=0.90，exact-match）；`sameStepIDs`（`internal/memorycompiler/runtime.go:1674`，调用点 L1659，confidence=0.90，exact-match）；`sameStringSet`（`internal/memorycompiler/runtime.go:1686`，调用点 L1662，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `limitStrings`（`internal/memorycompiler/runtime.go:969`，调用点 L1668/L1669/L1670，confidence=0.70，exact-match，3 条边，待源码复核）；`firstNonEmpty`（`internal/memorycompiler/runtime.go:2662`，调用点 L1656，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `service::sessionDelete`（`internal/acp/service.go:862`，调用点 L899，confidence=0.70，instance-method，待源码复核）；`Agent::Run`（`internal/agent/agent.go:675`，调用点 L696，confidence=0.80，instance-method，待源码复核）；`Controller::ClearSession`（`internal/control/controller.go:1977`，调用点 L1997，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/memorycompiler/runtime_test.go`
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/runtime_test.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`

#### 待修改项

- [ ] 先冻结 `validateIRExecution` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1641、L1654、L1655、L1662、L1665 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `validateIRExecution` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/runtime.go|validateIRExecution` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:1636-1672` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `validateIRExecution` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G19 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/memorycompiler/runtime_test.go`。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/runtime.go|validateIRExecution` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-315 · 圈复杂度 · `classifyStrategy`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-315 |
| Baseline key | `complexity\|internal/memorycompiler/runtime.go\|classifyStrategy` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 2796 |
| 函数/方法/包 | `classifyStrategy` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`classifyStrategy` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 2796-2810 行；可见控制流热点：L2798 `switch {`；L2799 `case strings.Contains(lower, "review") || strings.Contains(goal, "评审"):`；L2801 `case strings.Contains(lower, "bug") || strings.Contains(lower, "fix") || strings.Contains(goal, "修复…`；L2803 `case strings.Contains(lower, "frontend") || strings.Contains(lower, "ui") || strings.Contains(goal,…`；L2805 `case strings.Contains(lower, "goal") || strings.Contains(lower, "research") || strings.Contains(goa…`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function classifyStrategy（internal/memorycompiler/runtime.go:2796-2810）
- **可信生产调用方（4）：** `compressExecutionTrace`（`internal/memorycompiler/compression.go:412`，调用点 L420，confidence=0.90，exact-match）；`strategyScoreWithReason`（`internal/memorycompiler/runtime.go:1525`，调用点 L1544，confidence=0.90，exact-match）；`Runtime::writeTraceAndLearn`（`internal/memorycompiler/runtime.go:1829`，调用点 L1838，confidence=0.90，exact-match）；`updateGraph`（`internal/memorycompiler/runtime.go:2440`，调用点 L2455，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `classifyStrategy` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L2798、L2799、L2801、L2803、L2805 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `classifyStrategy` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/runtime.go|classifyStrategy` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:2796-2810` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `classifyStrategy` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G19 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/runtime.go|classifyStrategy` 下降或消失且无新增/恶化。
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
