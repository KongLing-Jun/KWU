# G19 · internal/memorycompiler · compression 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G19 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/memorycompiler` |
| 涉及文件 | `internal/memorycompiler/compression.go`<br>`internal/memorycompiler/runtime.go` |
| 主要治理目标 | 围绕 `internal/memorycompiler/runtime.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | memorycompiler 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 15 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 19 组；等待 G07 的同文件变更合并 |
| 前置依赖 | G07（internal/memorycompiler/runtime.go） |
| 后续验证重点 | 会话、状态或恢复语义 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **49/100**，对象 `internal/memorycompiler/compression.go` 的分项为严重度 10.0、集中度 5.9、影响面 21.4、可执行性 12.0；最高超限 1.93x，文件内 5 条问题，CodeGraph 记录 4 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 2 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G07。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `FL-015` | `file_lines\|internal/memorycompiler/compression.go\|` | 文件有效行数 | `internal/memorycompiler/compression.go` | `internal/memorycompiler/compression.go` | 1 | 1545/800 | +745（1.93x） | P2 | 4 可信 + 0 待核 caller / 4 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-303` | `complexity\|internal/memorycompiler/compression.go\|activeSemanticLayers` | 圈复杂度 | `internal/memorycompiler/compression.go` | `activeSemanticLayers` | 1365 | 25/10 | +15（2.50x） | P2 | 1 可信 + 0 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-302` | `complexity\|internal/memorycompiler/compression.go\|compressMemoryGraph` | 圈复杂度 | `internal/memorycompiler/compression.go` | `compressMemoryGraph` | 456 | 17/10 | +7（1.70x） | P3 | 1 可信 + 0 待核 caller / 3 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-304` | `complexity\|internal/memorycompiler/compression.go\|semanticOverlapSignals` | 圈复杂度 | `internal/memorycompiler/compression.go` | `semanticOverlapSignals` | 1400 | 17/10 | +7（1.70x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-301` | `complexity\|internal/memorycompiler/compression.go\|longTailCausalCandidates` | 圈复杂度 | `internal/memorycompiler/compression.go` | `longTailCausalCandidates` | 364 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-309` | `complexity\|internal/memorycompiler/runtime.go\|selectStrategy` | 圈复杂度 | `internal/memorycompiler/runtime.go` | `selectStrategy` | 1181 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `ND-014` | `nesting_depth\|internal/memorycompiler/runtime.go\|mutationsFromLearning` | 嵌套深度 | `internal/memorycompiler/runtime.go` | `mutationsFromLearning` | 2012 | 5/4 | +1（1.25x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-313` | `complexity\|internal/memorycompiler/runtime.go\|evaluateMutations` | 圈复杂度 | `internal/memorycompiler/runtime.go` | `evaluateMutations` | 2057 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-306` | `complexity\|internal/memorycompiler/runtime.go\|memoryCitationsForIR` | 圈复杂度 | `internal/memorycompiler/runtime.go` | `memoryCitationsForIR` | 726 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-314` | `complexity\|internal/memorycompiler/runtime.go\|mutationFeedbackInCooldown` | 圈复杂度 | `internal/memorycompiler/runtime.go` | `mutationFeedbackInCooldown` | 2197 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### FL-015 · 文件有效行数 · `internal/memorycompiler/compression.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-015 |
| Baseline key | `file_lines\|internal/memorycompiler/compression.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 49） |
| 文件 | `internal/memorycompiler/compression.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/memorycompiler/compression.go` |
| Baseline / 当前复测 / 阈值 | 1545 / 1545 / 800 |
| 超出 | +745，1.93x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/memorycompiler/compression.go` 当前有效代码行 **1545**，超过限制 **800**（+745）。CodeGraph 在文件中索引到的主要大符号包括 compressMemoryGraph@456-517、longTailCausalCandidates@364-410、crossGraphAlignment@519-560、buildCompressionReport@245-285、observerLoopReport@1020-1057、cloneCompressionReport@626-662；文件同时承载的调用影响覆盖 2 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 72 个函数/方法节点。
- **可信生产调用方（4）：** `learningTraceFor`（`internal/memorycompiler/runtime.go:1916`，调用点 L1937/L1928，confidence=0.90，exact-match，2 条边）；`updateGraph`（`internal/memorycompiler/runtime.go:2440`，调用点 L2534/L2535，confidence=0.90，exact-match，2 条边）；`executionTraceProjection`（`internal/memorycompiler/runtime.go:1891`，调用点 L1908，confidence=0.90，exact-match）；`Runtime::writeTraceAndLearn`（`internal/memorycompiler/runtime.go:1829`，调用点 L1866，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `canonicalStrings`（`internal/memorycompiler/runtime.go:963`，调用点 L504/L552/L553/L554，confidence=0.90，exact-match，18 条边）；`roundScore`（`internal/memorycompiler/runtime.go:994`，调用点 L268/L1159/L768/L877，confidence=0.90，exact-match，8 条边）；`classifyStrategy`（`internal/memorycompiler/runtime.go:2796`，调用点 L420，confidence=0.90，exact-match）；`graphRelation`（`internal/memorycompiler/runtime.go:2551`，调用点 L777，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `limitStrings`（`internal/memorycompiler/runtime.go:969`，调用点 L504/L552/L553/L554，confidence=0.70，exact-match，19 条边，待源码复核）；`containsString`（`internal/memorycompiler/runtime.go:2431`，调用点 L1470，confidence=0.70，exact-match，2 条边，待源码复核）；`firstNonEmpty`（`internal/memorycompiler/runtime.go:2662`，调用点 L420，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/memorycompiler/compression_test.go`
- **同目录测试候选：** `internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/compression.go`、`internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `internal/memorycompiler/compression.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `compressMemoryGraph`、`longTailCausalCandidates`、`crossGraphAlignment`、`buildCompressionReport`、`observerLoopReport` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-301、CX-302、CX-303、CX-304。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/memorycompiler/compression.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/compression.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/memorycompiler/compression.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/memorycompiler/compression_test.go`。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/compression.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/memorycompiler/compression.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-303 · 圈复杂度 · `activeSemanticLayers`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-303 |
| Baseline key | `complexity\|internal/memorycompiler/compression.go\|activeSemanticLayers` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 45） |
| 文件 | `internal/memorycompiler/compression.go` |
| 代码行 | 1365 |
| 函数/方法/包 | `activeSemanticLayers` |
| Baseline / 当前复测 / 阈值 | 25 / 25 / 10 |
| 超出 | +15，2.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`activeSemanticLayers` 的圈复杂度当前为 **25**，超过限制 **10**（+15，2.50x）。定义范围为 1365-1398 行；可见控制流热点：L1367 `if causal.TotalEdges > 0 || causal.RetainedEdges > 0 {`；L1373 `if control.EquilibriumState != "" || len(control.EquilibriumActions) > 0 {`；L1379 `if alignment.Status != "" || alignment.AbstractionLevel != "" {`；L1382 `if dynamics.EntropyBand != "" || dynamics.AmplitudeBand != "" || dynamics.OverRegularized {`；L1388 `if observer.PredictionBias.Mode != "" || len(observer.PredictionBias.CounterfactualChecks) > 0 {`；L1394 `if observer.LongTailSafety.Mode != "" || len(observer.Damping.SuppressedSignals) > 0 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function activeSemanticLayers（internal/memorycompiler/compression.go:1365-1398）
- **可信生产调用方（1）：** `layerCollapseReport`（`internal/memorycompiler/compression.go:1345`，调用点 L1346，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `canonicalStrings`（`internal/memorycompiler/runtime.go:963`，调用点 L1397，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `limitStrings`（`internal/memorycompiler/runtime.go:969`，调用点 L1397，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/compression.go`、`internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `activeSemanticLayers` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1367、L1373、L1382、L1388、L1394 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `activeSemanticLayers` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/compression.go|activeSemanticLayers` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/compression.go:1365-1398` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `activeSemanticLayers` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/compression.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/compression.go|activeSemanticLayers` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-302 · 圈复杂度 · `compressMemoryGraph`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-302 |
| Baseline key | `complexity\|internal/memorycompiler/compression.go\|compressMemoryGraph` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 43） |
| 文件 | `internal/memorycompiler/compression.go` |
| 代码行 | 456 |
| 函数/方法/包 | `compressMemoryGraph` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`compressMemoryGraph` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 456-517 行；可见控制流热点：L457 `if now.IsZero() {`；L472 `if node.Quality == QualityNoise || node.Quality == QualityCorrupted {`；L479 `for _, edge := range st.Edges {`；L493 `if pi != pj {`；L499 `if !nodes[i].Timestamp.Equal(nodes[j].Timestamp) {`；L511 `if len(anchors) >= maxCompressionStrings {`。CodeGraph 记录的下游职责候选主要涉及 任务/并发（truthLockedImportance）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function compressMemoryGraph（internal/memorycompiler/compression.go:456-517）
- **可信生产调用方（1）：** `buildCompressionReport`（`internal/memorycompiler/compression.go:245`，调用点 L252，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `memoryNodeCompressionPriority`（`internal/memorycompiler/compression.go:731`，调用点 L491/L492，confidence=0.90，exact-match，2 条边）；`truthLockedImportance`（`internal/memorycompiler/compression.go:746`，调用点 L475，confidence=0.90，exact-match）；`canonicalStrings`（`internal/memorycompiler/runtime.go:963`，调用点 L504，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `limitStrings`（`internal/memorycompiler/runtime.go:969`，调用点 L504，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/memorycompiler/compression_test.go`
- **同目录测试候选：** `internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/compression.go`、`internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `compressMemoryGraph` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L457、L472、L485、L499、L511 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `compressMemoryGraph` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/compression.go|compressMemoryGraph` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/compression.go:456-517` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `compressMemoryGraph` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/memorycompiler/compression_test.go`。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/compression.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/compression.go|compressMemoryGraph` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-304 · 圈复杂度 · `semanticOverlapSignals`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-304 |
| Baseline key | `complexity\|internal/memorycompiler/compression.go\|semanticOverlapSignals` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 41） |
| 文件 | `internal/memorycompiler/compression.go` |
| 代码行 | 1400 |
| 函数/方法/包 | `semanticOverlapSignals` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`semanticOverlapSignals` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 1400-1424 行；可见控制流热点：L1402 `if control.Controller != "" && (control.EquilibriumState != "" || len(control.EquilibriumActions) >…`；L1405 `if observer.AdvisoryBridge.AdvisoryEligible && observer.PredictionBias.Mode != "" {`；L1408 `if observer.TemporalSync.Clock != "" && observer.TemporalVariance.Mode != "" {`；L1414 `if causal.TotalEdges > 0 && len(memory.AnchorNodes) > 0 && alignment.Status != "empty" {`；L1417 `if observer.LongTailSafety.LongTailPreserved && observer.AdvisoryBridge.AdvisoryEligible {`；L1420 `if memory.ConflictCount > 0 && control.OscillationBand != "none" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function semanticOverlapSignals（internal/memorycompiler/compression.go:1400-1424）
- **可信生产调用方（1）：** `layerCollapseReport`（`internal/memorycompiler/compression.go:1345`，调用点 L1347，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `canonicalStrings`（`internal/memorycompiler/runtime.go:963`，调用点 L1423，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `limitStrings`（`internal/memorycompiler/runtime.go:969`，调用点 L1423，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/compression.go`、`internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `semanticOverlapSignals` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1402、L1408、L1411、L1417、L1420 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `semanticOverlapSignals` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/compression.go|semanticOverlapSignals` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/compression.go:1400-1424` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `semanticOverlapSignals` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/compression.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/compression.go|semanticOverlapSignals` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-301 · 圈复杂度 · `longTailCausalCandidates`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-301 |
| Baseline key | `complexity\|internal/memorycompiler/compression.go\|longTailCausalCandidates` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/memorycompiler/compression.go` |
| 代码行 | 364 |
| 函数/方法/包 | `longTailCausalCandidates` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`longTailCausalCandidates` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 364-410 行；可见控制流热点：L365 `if limit <= 2 || len(relationCounts) <= 1 {`；L374 `if longTailBudget < 1 {`；L384 `if ci != cj {`；L389 `if pi != pj {`；L396 `if covered[edge.Relation] {`；L405 `if len(out) >= longTailBudget {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function longTailCausalCandidates（internal/memorycompiler/compression.go:364-410）
- **可信生产调用方（1）：** `selectCausalAnchors`（`internal/memorycompiler/compression.go:330`，调用点 L335，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `causalEdgeKey`（`internal/memorycompiler/compression.go:708`，调用点 L392/L399，confidence=0.90，exact-match，3 条边）；`causalEdgePriority`（`internal/memorycompiler/compression.go:689`，调用点 L387/L388，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/compression.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `longTailCausalCandidates` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L365、L374、L389、L396、L405 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `longTailCausalCandidates` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/compression.go|longTailCausalCandidates` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/compression.go:364-410` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `longTailCausalCandidates` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/compression.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/compression.go|longTailCausalCandidates` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-309 · 圈复杂度 · `selectStrategy`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-309 |
| Baseline key | `complexity\|internal/memorycompiler/runtime.go\|selectStrategy` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 1181 |
| 函数/方法/包 | `selectStrategy` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`selectStrategy` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 1181-1230 行；可见控制流热点：L1183 `if len(explorationRates) > 0 {`；L1195 `if !lowSuccessStrategy(candidate.strategy) {`；L1201 `if explore, candidate := explorationCandidate(goal, eligible, explorationRate); explore {`；L1208 `if pick.Mode == "explore" {`；L1213 `if candidate.strategy.ID == pick.Selected {`；L1225 `if len(pick.Rejected) >= 3 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function selectStrategy（internal/memorycompiler/runtime.go:1181-1230）
- **可信生产调用方（1）：** `buildIRWithPolicy`（`internal/memorycompiler/runtime.go:544`，调用点 L556，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `clampExplorationRatePercent`（`internal/memorycompiler/runtime.go:1260`，调用点 L1182/L1184，confidence=0.90，exact-match，2 条边）；`lowSuccessStrategy`（`internal/memorycompiler/runtime.go:1588`，调用点 L1195/L1217，confidence=0.90，exact-match，2 条边）；`explorationCandidate`（`internal/memorycompiler/runtime.go:1232`，调用点 L1201，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `buildIR`（`internal/memorycompiler/runtime.go:539`，调用点 L540，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/memorycompiler/runtime_test.go`
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/runtime_test.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`

#### 待修改项

- [ ] 先冻结 `selectStrategy` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1183、L1195、L1208、L1213、L1225 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `selectStrategy` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/runtime.go|selectStrategy` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:1181-1230` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `selectStrategy` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/memorycompiler/runtime_test.go`。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/runtime.go|selectStrategy` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-014 · 嵌套深度 · `mutationsFromLearning`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-014 |
| Baseline key | `nesting_depth\|internal/memorycompiler/runtime.go\|mutationsFromLearning` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 2012 |
| 函数/方法/包 | `mutationsFromLearning` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`mutationsFromLearning` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L2015 `for _, reason := range learning.CompilerImprovements {`；L2018 `if strings.Contains(reason, "source-of-truth") {`；L2041 `for _, pattern := range learning.MemoryNoisePatterns {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function mutationsFromLearning（internal/memorycompiler/runtime.go:2012-2055）
- **可信生产调用方（1）：** `Runtime::writeTraceAndLearn`（`internal/memorycompiler/runtime.go:1829`，调用点 L1854，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `mutationsFromLearning` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 从 L2015、L2018、L2041 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/memorycompiler/runtime.go|mutationsFromLearning` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:2012-2055` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `mutationsFromLearning` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/memorycompiler/runtime.go|mutationsFromLearning` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-313 · 圈复杂度 · `evaluateMutations`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-313 |
| Baseline key | `complexity\|internal/memorycompiler/runtime.go\|evaluateMutations` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 2057 |
| 函数/方法/包 | `evaluateMutations` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`evaluateMutations` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 2057-2105 行；可见控制流热点：L2058 `if len(existing) == 0 {`；L2064 `for i := range existing {`；L2066 `if !m.Applied || m.Status == "accepted" || m.Status == "rejected" {`；L2072 `if containsString(m.EvaluationTraceIDs, tr.ID) || containsString(m.EvidenceTraceIDs, tr.ID) {`；L2081 `if trials >= mutationMinEvalTrials {`；L2082 `if m.EvaluationScore >= mutationAcceptThreshold && m.EvaluationScore+mutationRegressionMargin >= m.…`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function evaluateMutations（internal/memorycompiler/runtime.go:2057-2105）
- **可信生产调用方（1）：** `Runtime::writeTraceAndLearn`（`internal/memorycompiler/runtime.go:1829`，调用点 L1841，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `averageEvaluationScore`（`internal/memorycompiler/runtime.go:2107`，调用点 L2077，confidence=0.90，exact-match）；`traceQualityScore`（`internal/memorycompiler/runtime.go:2114`，调用点 L2062，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `containsString`（`internal/memorycompiler/runtime.go:2431`，调用点 L2072，confidence=0.70，exact-match，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `evaluateMutations` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L2058、L2066、L2069、L2081、L2082 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `evaluateMutations` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/runtime.go|evaluateMutations` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:2057-2105` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `evaluateMutations` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/runtime.go|evaluateMutations` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-306 · 圈复杂度 · `memoryCitationsForIR`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-306 |
| Baseline key | `complexity\|internal/memorycompiler/runtime.go\|memoryCitationsForIR` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 726 |
| 函数/方法/包 | `memoryCitationsForIR` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`memoryCitationsForIR` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 726-780 行；可见控制流热点：L735 `if c.Source == "" {`；L739 `if c.Note == "" || seen[key] || len(out) >= 5 {`；L747 `if ref.Influence != "" {`；L750 `if ref.Quality != "" {`；L762 `if c.Source != "" {`；L772 `for _, note := range ir.RiskNotes {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function memoryCitationsForIR（internal/memorycompiler/runtime.go:726-780）
- **可信生产调用方（1）：** `Runtime::StartTurn`（`internal/memorycompiler/runtime.go:431`，调用点 L449，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `canonicalizeIR`（`internal/memorycompiler/runtime.go:789`，调用点 L727，confidence=0.90，exact-match）；`summarizeText`（`internal/memorycompiler/runtime.go:982`，调用点 L733，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L692，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `memoryCitationsForIR` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L735、L745、L750、L760、L772 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `memoryCitationsForIR` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/runtime.go|memoryCitationsForIR` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:726-780` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `memoryCitationsForIR` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/runtime.go|memoryCitationsForIR` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-314 · 圈复杂度 · `mutationFeedbackInCooldown`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-314 |
| Baseline key | `complexity\|internal/memorycompiler/runtime.go\|mutationFeedbackInCooldown` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/memorycompiler/runtime.go` |
| 代码行 | 2197 |
| 函数/方法/包 | `mutationFeedbackInCooldown` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`mutationFeedbackInCooldown` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 2197-2227 行；可见控制流热点：L2198 `if next.CreatedAt.IsZero() {`；L2204 `for _, m := range existing {`；L2205 `if m.Target != next.Target || m.Change != next.Change {`；L2212 `if ref.IsZero() {`；L2215 `if ref.IsZero() {`；L2222 `if delta < cooldown {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function mutationFeedbackInCooldown（internal/memorycompiler/runtime.go:2197-2227）
- **可信生产调用方（1）：** `mergeMutationsWithPolicy`（`internal/memorycompiler/runtime.go:2151`，调用点 L2167，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `mergeMutations`（`internal/memorycompiler/runtime.go:2147`，调用点 L2148，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`
- **需同步检查：** `internal/memorycompiler/runtime.go`、`internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`

#### 待修改项

- [ ] 先冻结 `mutationFeedbackInCooldown` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L2198、L2204、L2208、L2215、L2222 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `mutationFeedbackInCooldown` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memorycompiler/runtime.go|mutationFeedbackInCooldown` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memorycompiler/runtime.go:2197-2227` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `mutationFeedbackInCooldown` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memorycompiler/compression_test.go`、`internal/memorycompiler/hardening_regression_test.go`、`internal/memorycompiler/review_fixes_test.go`、`internal/memorycompiler/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memorycompiler`。
- [ ] 运行相关包 vet：`go vet ./internal/memorycompiler`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memorycompiler/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memorycompiler/runtime.go|mutationFeedbackInCooldown` 下降或消失且无新增/恶化。
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
