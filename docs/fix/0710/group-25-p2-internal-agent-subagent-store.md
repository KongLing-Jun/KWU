# G25 · internal/agent · subagent_store 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G25 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/agent` |
| 涉及文件 | `internal/agent/parallel_tasks.go`<br>`internal/agent/reasoning_language.go`<br>`internal/agent/subagent_store.go`<br>`internal/agent/prune.go` |
| 主要治理目标 | 围绕 `internal/agent/parallel_tasks.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | agent 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 5 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 25 组；等待 G05 的同文件变更合并 |
| 前置依赖 | G05（internal/agent/parallel_tasks.go） |
| 后续验证重点 | 并发、取消或事件顺序；会话、状态或恢复语义；协议、序列化或流式事件兼容；配置、平台或向后兼容；持久化、迁移或文件格式兼容；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **45/100**，对象 `*ParallelTasksTool.runAsBackgroundJobs` 的分项为严重度 6.6、集中度 5.9、影响面 20.3、可执行性 12.0；最高超限 1.70x，文件内 5 条问题，CodeGraph 记录 0 个可信生产调用方、1 个待核候选。

### 治理边界与组间关系

本组涉及 4 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G05。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-042` | `complexity\|internal/agent/parallel_tasks.go\|*ParallelTasksTool.runAsBackgroundJobs` | 圈复杂度 | `internal/agent/parallel_tasks.go` | `*ParallelTasksTool.runAsBackgroundJobs` | 409 | 17/10 | +7（1.70x） | P2 | 0 可信 + 1 待核 caller / 4 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-045` | `complexity\|internal/agent/reasoning_language.go\|hasLeadingInjectedBlock` | 圈复杂度 | `internal/agent/reasoning_language.go` | `hasLeadingInjectedBlock` | 87 | 15/10 | +5（1.50x） | P2 | 2 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-051` | `complexity\|internal/agent/subagent_store.go\|validateMeta` | 圈复杂度 | `internal/agent/subagent_store.go` | `validateMeta` | 629 | 13/10 | +3（1.30x） | P2 | 5 可信 + 0 待核 caller / 2 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-048` | `complexity\|internal/agent/subagent_store.go\|ListSubagentsByParent` | 圈复杂度 | `internal/agent/subagent_store.go` | `ListSubagentsByParent` | 146 | 12/10 | +2（1.20x） | P2 | 6 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-044` | `complexity\|internal/agent/prune.go\|*Agent.PruneStaleToolResults` | 圈复杂度 | `internal/agent/prune.go` | `*Agent.PruneStaleToolResults` | 29 | 14/10 | +4（1.40x） | P3 | 1 可信 + 3 待核 caller / 2 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-050` | `complexity\|internal/agent/subagent_store.go\|*SubagentStore.prepareFork` | 圈复杂度 | `internal/agent/subagent_store.go` | `*SubagentStore.prepareFork` | 471 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 4 可信 + 5 待核 callee / 0 unresolved | 待修改 |
| `CX-052` | `complexity\|internal/agent/subagent_store.go\|validSubagentRef` | 圈复杂度 | `internal/agent/subagent_store.go` | `validSubagentRef` | 834 | 11/10 | +1（1.10x） | P3 | 4 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-043` | `complexity\|internal/agent/parallel_tasks.go\|validateParallelTaskItems` | 圈复杂度 | `internal/agent/parallel_tasks.go` | `validateParallelTaskItems` | 497 | 13/10 | +3（1.30x） | P3 | 2 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-049` | `complexity\|internal/agent/subagent_store.go\|*SubagentStore.CleanupStaleRunning` | 圈复杂度 | `internal/agent/subagent_store.go` | `*SubagentStore.CleanupStaleRunning` | 210 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-041` | `complexity\|internal/agent/parallel_tasks.go\|formatParallelTasksAggregate` | 圈复杂度 | `internal/agent/parallel_tasks.go` | `formatParallelTasksAggregate` | 368 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-042 · 圈复杂度 · `*ParallelTasksTool.runAsBackgroundJobs`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-042 |
| Baseline key | `complexity\|internal/agent/parallel_tasks.go\|*ParallelTasksTool.runAsBackgroundJobs` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 45） |
| 文件 | `internal/agent/parallel_tasks.go` |
| 代码行 | 409 |
| 函数/方法/包 | `*ParallelTasksTool.runAsBackgroundJobs` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*ParallelTasksTool.runAsBackgroundJobs` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 409-495 行；可见控制流热点：L411 `if !ok {`；L428 `if strings.TrimSpace(t.Prompt) == "" {`；L444 `if t.MaxSteps > 0 {`；L455 `if err != nil {`；L473 `for _, r := range refs {`；L488 `if r.Label != "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（validateParallelTaskItems）、状态与控制（ParallelTasksTool::Execute）、任务/并发（validateParallelTaskItems、extractJobID、ParallelTasksTool::Execute、Manager::WaitForSession）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method ParallelTasksTool::runAsBackgroundJobs（internal/agent/parallel_tasks.go:409-495）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `ParallelTasksTool::Execute`（`internal/agent/parallel_tasks.go:111`，调用点 L131，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（4）：** `validateParallelTaskItems`（`internal/agent/parallel_tasks.go:497`，调用点 L416，confidence=0.90，exact-match）；`extractJobID`（`internal/agent/parallel_tasks.go:548`，调用点 L463，confidence=0.90，exact-match）；`FromContext`（`internal/jobs/jobs.go:1485`，调用点 L410，confidence=0.90，import）；`SessionFromContext`（`internal/jobs/jobs.go:1498`，调用点 L414，confidence=0.90，import）
- **待源码复核被调候选（2）：** `ParallelTasksTool::Execute`（`internal/agent/parallel_tasks.go:111`，调用点 L459，confidence=0.70，exact-match，待源码复核）；`Manager::WaitForSession`（`internal/jobs/jobs.go:708`，调用点 L478，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `ParallelTasksTool::Execute`（`internal/agent/parallel_tasks.go:111`，调用点 L131，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/agent`、`internal/jobs`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/parallel_tasks.go`、`internal/jobs/jobs.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*ParallelTasksTool.runAsBackgroundJobs` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L411、L432、L450、L467、L488 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*ParallelTasksTool.runAsBackgroundJobs` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/parallel_tasks.go|*ParallelTasksTool.runAsBackgroundJobs` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/parallel_tasks.go:409-495` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*ParallelTasksTool.runAsBackgroundJobs` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/parallel_tasks.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/parallel_tasks.go|*ParallelTasksTool.runAsBackgroundJobs` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-045 · 圈复杂度 · `hasLeadingInjectedBlock`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-045 |
| Baseline key | `complexity\|internal/agent/reasoning_language.go\|hasLeadingInjectedBlock` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 45） |
| 文件 | `internal/agent/reasoning_language.go` |
| 代码行 | 87 |
| 函数/方法/包 | `hasLeadingInjectedBlock` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`hasLeadingInjectedBlock` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 87-127 行；可见控制流热点：L89 `for {`；L91 `case strings.HasPrefix(s, "<"+target+">"):`；L99 `case target != "reasoning-language" && strings.HasPrefix(s, "<reasoning-language>"):`；L105 `case strings.HasPrefix(s, "<memory-update>"):`；L114 `if !ok {`；L120 `if !ok {`。CodeGraph 记录的下游职责候选主要涉及 任务/并发（trimLeadingTransientBlock）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function hasLeadingInjectedBlock（internal/agent/reasoning_language.go:87-127）
- **可信生产调用方（2）：** `WithResponseLanguage`（`internal/agent/reasoning_language.go:67`，调用点 L69，confidence=0.90，exact-match）；`WithReasoningLanguage`（`internal/agent/reasoning_language.go:79`，调用点 L81，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `trimLeadingTransientBlock`（`internal/agent/reasoning_language.go:129`，调用点 L95/L101/L107/L113，confidence=0.90，exact-match，5 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::Compose`（`internal/control/input.go:165`，调用点 L181，confidence=0.90，import，待源码复核）；`Controller::ComposeSynthetic`（`internal/control/input.go:234`，调用点 L239，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/reasoning_language.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `hasLeadingInjectedBlock` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L89、L93、L102、L111、L120 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `hasLeadingInjectedBlock` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/reasoning_language.go|hasLeadingInjectedBlock` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/reasoning_language.go:87-127` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `hasLeadingInjectedBlock` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/reasoning_language.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/reasoning_language.go|hasLeadingInjectedBlock` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-051 · 圈复杂度 · `validateMeta`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-051 |
| Baseline key | `complexity\|internal/agent/subagent_store.go\|validateMeta` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 45） |
| 文件 | `internal/agent/subagent_store.go` |
| 代码行 | 629 |
| 函数/方法/包 | `validateMeta` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`validateMeta` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 629-660 行；可见控制流热点：L630 `if meta.Status == SubagentRunning {`；L636 `if meta.Status == SubagentInterrupted {`；L641 `case meta.Kind != want.Kind:`；L647 `case meta.SystemPromptHash != want.SystemPromptHash:`；L651 `case meta.ToolSchemaHash != want.ToolSchemaHash:`；L656 `if err := validateActorMeta(meta, want); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（validateActorMeta）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function validateMeta（internal/agent/subagent_store.go:629-660）
- **可信生产调用方（5）：** `SubagentStore::PrepareContinue`（`internal/agent/subagent_store.go:268`，调用点 L296，confidence=0.90，exact-match）；`SubagentStore::PrepareLegacyForkFrom`（`internal/agent/subagent_store.go:310`，调用点 L338，confidence=0.90，exact-match）；`SubagentStore::nearestLineageSource`（`internal/agent/subagent_store.go:371`，调用点 L391，confidence=0.90，exact-match）；`SubagentStore::compatibleCopiesFromSource`（`internal/agent/subagent_store.go:453`，调用点 L463，confidence=0.90，exact-match）；`SubagentStore::prepareFork`（`internal/agent/subagent_store.go:471`，调用点 L495，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `metaFromSpec`（`internal/agent/subagent_store.go:586`，调用点 L639，confidence=0.90，exact-match）；`validateActorMeta`（`internal/agent/subagent_store.go:662`，调用点 L656，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `sameStrings`（`internal/agent/subagent_store.go:863`，调用点 L649，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `Build`（`internal/boot/boot.go:152`，调用点 L802，confidence=0.80，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/subagent_store.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `validateMeta` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L630、L640、L645、L649、L656 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `validateMeta` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/subagent_store.go|validateMeta` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/subagent_store.go:629-660` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `validateMeta` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/subagent_store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/subagent_store.go|validateMeta` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-048 · 圈复杂度 · `ListSubagentsByParent`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-048 |
| Baseline key | `complexity\|internal/agent/subagent_store.go\|ListSubagentsByParent` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 44） |
| 文件 | `internal/agent/subagent_store.go` |
| 代码行 | 146 |
| 函数/方法/包 | `ListSubagentsByParent` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ListSubagentsByParent` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 146-188 行；可见控制流热点：L148 `if strings.TrimSpace(sessionDir) == "" || parentSession == "" {`；L154 `if os.IsNotExist(err) {`；L160 `for _, entry := range entries {`；L165 `if !validSubagentRef(ref) {`；L170 `if err != nil {`；L177 `if strings.TrimSpace(meta.ParentSession) != parentSession {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ListSubagentsByParent（internal/agent/subagent_store.go:146-188）
- **可信生产调用方（6）：** `taskSubagentHistoriesByToolCallID`（`desktop/app.go:3547`，调用点 L3548，confidence=0.90，import）；`trashSubagentArtifacts`（`desktop/sessions.go:581`，调用点 L582，confidence=0.90，import）；`copySubagentArtifacts`（`internal/agent/migrate.go:810`，调用点 L814，confidence=0.90，exact-match）；`DeleteSubagentsByParent`（`internal/agent/subagent_store.go:192`，调用点 L193，confidence=0.90，exact-match）；`SubagentStore::nearestLineageSource`（`internal/agent/subagent_store.go:371`，调用点 L377，confidence=0.90，exact-match）；`SubagentStore::compatibleCopiesFromSource`（`internal/agent/subagent_store.go:453`，调用点 L454，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `validSubagentRef`（`internal/agent/subagent_store.go:834`，调用点 L165，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/agent`、`desktop`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/subagent_store.go`、`desktop/app.go`、`desktop/sessions.go`、`internal/agent/migrate.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `ListSubagentsByParent` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L148、L154、L161、L170、L177 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `ListSubagentsByParent` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/subagent_store.go|ListSubagentsByParent` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/subagent_store.go:146-188` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ListSubagentsByParent` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/subagent_store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/subagent_store.go|ListSubagentsByParent` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-044 · 圈复杂度 · `*Agent.PruneStaleToolResults`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-044 |
| Baseline key | `complexity\|internal/agent/prune.go\|*Agent.PruneStaleToolResults` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 43） |
| 文件 | `internal/agent/prune.go` |
| 代码行 | 29 |
| 函数/方法/包 | `*Agent.PruneStaleToolResults` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Agent.PruneStaleToolResults` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 29-82 行；可见控制流热点：L31 `if a.contextWindow <= 0 {`；L40 `for i := head; i < start; i++ {`；L51 `if a.keepPolicy&KeepErrors != 0 && isErrorMessage(m) {`；L56 `if len(idx) == 0 {`；L61 `for _, i := range idx {`；L71 `for _, i := range idx {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（archiveMessages）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::PruneStaleToolResults（internal/agent/prune.go:29-82）
- **可信生产调用方（1）：** `Controller::maybeColdResumePrune`（`internal/control/controller.go:2538`，调用点 L2553，confidence=0.90，exact-match）
- **待源码复核调用方候选（3）：** `resume`（`benchmarks/context-maintenance-e2e/main.go:126`，调用点 L150，confidence=0.70，instance-method，待源码复核）；`comprehension`（`benchmarks/context-maintenance-e2e/main.go:180`，调用点 L211，confidence=0.70，instance-method，待源码复核）；`Agent::maybeCompact`（`internal/agent/compact.go:84`，调用点 L111，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `isErrorMessage`（`internal/agent/compact.go:458`，调用点 L51，confidence=0.90，exact-match）；`archiveMessages`（`internal/agent/compact.go:722`，调用点 L64，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Agent::planCompaction`（`internal/agent/compact.go:505`，调用点 L35，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `main`（`benchmarks/context-maintenance-e2e/main.go:240`，调用点 L248，confidence=0.70，exact-match，待源码复核）；`Agent::Run`（`internal/agent/agent.go:675`，调用点 L806，confidence=0.70，instance-method，待源码复核）；`Controller::Resume`（`internal/control/controller.go:2442`，调用点 L2455，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/agent`、`internal/control`、`benchmarks`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/prune_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/prune.go`、`internal/control/controller.go`、`benchmarks/context-maintenance-e2e/main.go`、`internal/agent/compact.go`、`internal/agent/prune_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.PruneStaleToolResults` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L31、L40、L56、L61、L71 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Agent.PruneStaleToolResults` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/prune.go|*Agent.PruneStaleToolResults` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/prune.go:29-82` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.PruneStaleToolResults` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/prune_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/prune.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/prune.go|*Agent.PruneStaleToolResults` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-050 · 圈复杂度 · `*SubagentStore.prepareFork`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-050 |
| Baseline key | `complexity\|internal/agent/subagent_store.go\|*SubagentStore.prepareFork` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/agent/subagent_store.go` |
| 代码行 | 471 |
| 函数/方法/包 | `*SubagentStore.prepareFork` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*SubagentStore.prepareFork` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 471-521 行；可见控制流热点：L472 `if s == nil {`；L479 `if sourceRef == "" {`；L487 `if err != nil {`；L495 `if err := validateMeta(meta, spec); err != nil {`；L504 `if err != nil {`；L514 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（LoadSession、validateMeta、SubagentStore::LoadMeta、SubagentStore::validateForkOwner）、持久化/I-O（SubagentStore::lock、SubagentStore::LoadMeta、SubagentStore::validateForkOwner、SubagentStore::newRef）、任务/并发（SubagentStore::lock）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method SubagentStore::prepareFork（internal/agent/subagent_store.go:471-521）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `SubagentStore::prepareContinueFromAncestor`（`internal/agent/subagent_store.go:347`，调用点 L358，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（4）：** `LoadSession`（`internal/agent/save.go:62`，调用点 L503，confidence=0.90，exact-match）；`metaFromSpec`（`internal/agent/subagent_store.go:586`，调用点 L518，confidence=0.90，exact-match）；`validateMeta`（`internal/agent/subagent_store.go:629`，调用点 L495，confidence=0.90，exact-match）；`requireParentSession`（`internal/agent/subagent_store.go:703`，调用点 L475，confidence=0.90，exact-match）
- **待源码复核被调候选（5）：** `SubagentStore::lock`（`internal/agent/subagent_store.go:773`，调用点 L482/L513，confidence=0.70，instance-method，2 条边，待源码复核）；`SubagentStore::LoadMeta`（`internal/agent/subagent_store.go:571`，调用点 L486，confidence=0.70，instance-method，待源码复核）；`SubagentStore::validateForkOwner`（`internal/agent/subagent_store.go:722`，调用点 L499，confidence=0.70，instance-method，待源码复核）；`SubagentStore::newRef`（`internal/agent/subagent_store.go:790`，调用点 L509，confidence=0.70，instance-method，待源码复核）；`SubagentStore::sessionPath`（`internal/agent/subagent_store.go:798`，调用点 L503，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/subagent_store_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/subagent_store.go`、`internal/agent/save.go`、`internal/agent/subagent_store_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`

#### 待修改项

- [ ] 先冻结 `*SubagentStore.prepareFork` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L472、L483、L491、L504、L514 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*SubagentStore.prepareFork` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/subagent_store.go|*SubagentStore.prepareFork` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/subagent_store.go:471-521` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*SubagentStore.prepareFork` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/subagent_store_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/subagent_store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/subagent_store.go|*SubagentStore.prepareFork` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-052 · 圈复杂度 · `validSubagentRef`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-052 |
| Baseline key | `complexity\|internal/agent/subagent_store.go\|validSubagentRef` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/agent/subagent_store.go` |
| 代码行 | 834 |
| 函数/方法/包 | `validSubagentRef` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`validSubagentRef` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 834-845 行；可见控制流热点：L835 `if !strings.HasPrefix(ref, "sa_") {`；L838 `for _, r := range ref {`；L839 `if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '…`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function validSubagentRef（internal/agent/subagent_store.go:834-845）
- **可信生产调用方（4）：** `ListSubagentsByParent`（`internal/agent/subagent_store.go:146`，调用点 L165，confidence=0.90，exact-match）；`SubagentStore::CleanupStaleRunning`（`internal/agent/subagent_store.go:210`，调用点 L228，confidence=0.90，exact-match）；`SubagentStore::LoadMeta`（`internal/agent/subagent_store.go:571`，调用点 L573，confidence=0.90，exact-match）；`SubagentStore::lock`（`internal/agent/subagent_store.go:773`，调用点 L774，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/subagent_store.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `validSubagentRef` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L835、L838、L839 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `validSubagentRef` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/subagent_store.go|validSubagentRef` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/subagent_store.go:834-845` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `validSubagentRef` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/subagent_store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/subagent_store.go|validSubagentRef` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-043 · 圈复杂度 · `validateParallelTaskItems`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-043 |
| Baseline key | `complexity\|internal/agent/parallel_tasks.go\|validateParallelTaskItems` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/agent/parallel_tasks.go` |
| 代码行 | 497 |
| 函数/方法/包 | `validateParallelTaskItems` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`validateParallelTaskItems` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 497-536 行；可见控制流热点：L498 `for i, t := range tasks {`；L502 `for j, dep := range t.DependsOn {`；L506 `if dep == i {`；L518 `case 2:`；L523 `if err := visit(dep); err != nil {`；L531 `if err := visit(i); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function validateParallelTaskItems（internal/agent/parallel_tasks.go:497-536）
- **可信生产调用方（2）：** `ParallelTasksTool::Execute`（`internal/agent/parallel_tasks.go:111`，调用点 L124，confidence=0.90，exact-match）；`ParallelTasksTool::runAsBackgroundJobs`（`internal/agent/parallel_tasks.go:409`，调用点 L416，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `ParallelTasksTool::Execute`（`internal/agent/parallel_tasks.go:111`，调用点 L124，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/parallel_tasks.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `validateParallelTaskItems` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L498、L503、L516、L522、L531 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `validateParallelTaskItems` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/parallel_tasks.go|validateParallelTaskItems` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/parallel_tasks.go:497-536` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `validateParallelTaskItems` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/parallel_tasks.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/parallel_tasks.go|validateParallelTaskItems` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-049 · 圈复杂度 · `*SubagentStore.CleanupStaleRunning`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-049 |
| Baseline key | `complexity\|internal/agent/subagent_store.go\|*SubagentStore.CleanupStaleRunning` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/agent/subagent_store.go` |
| 代码行 | 210 |
| 函数/方法/包 | `*SubagentStore.CleanupStaleRunning` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*SubagentStore.CleanupStaleRunning` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 210-246 行；可见控制流热点：L211 `if s == nil {`；L216 `if os.IsNotExist(err) {`；L223 `for _, entry := range entries {`；L228 `if !validSubagentRef(ref) {`；L232 `if err != nil {`；L240 `if err := s.saveMeta(meta); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（SubagentStore::LoadMeta）、持久化/I-O（SubagentStore::LoadMeta、SubagentStore::saveMeta）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method SubagentStore::CleanupStaleRunning（internal/agent/subagent_store.go:210-246）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `newSubagentStore`（`internal/boot/boot.go:1442`，调用点 L1448，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `validSubagentRef`（`internal/agent/subagent_store.go:834`，调用点 L228，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `SubagentStore::LoadMeta`（`internal/agent/subagent_store.go:571`，调用点 L231，confidence=0.70，instance-method，待源码复核）；`SubagentStore::saveMeta`（`internal/agent/subagent_store.go:801`，调用点 L240，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Build`（`internal/boot/boot.go:152`，调用点 L571，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/agent`、`internal/boot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/subagent_store_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/subagent_store.go`、`internal/boot/boot.go`、`internal/agent/subagent_store_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`

#### 待修改项

- [ ] 先冻结 `*SubagentStore.CleanupStaleRunning` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L211、L216、L224、L232、L240 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*SubagentStore.CleanupStaleRunning` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/subagent_store.go|*SubagentStore.CleanupStaleRunning` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/subagent_store.go:210-246` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*SubagentStore.CleanupStaleRunning` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/subagent_store_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/subagent_store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/subagent_store.go|*SubagentStore.CleanupStaleRunning` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-041 · 圈复杂度 · `formatParallelTasksAggregate`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-041 |
| Baseline key | `complexity\|internal/agent/parallel_tasks.go\|formatParallelTasksAggregate` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/agent/parallel_tasks.go` |
| 代码行 | 368 |
| 函数/方法/包 | `formatParallelTasksAggregate` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`formatParallelTasksAggregate` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 368-406 行；可见控制流热点：L371 `if cancelled {`；L374 `if st == parallelTaskCompleted {`；L384 `switch st {`；L387 `case parallelTaskCancelled:`；L393 `case parallelTaskSkipped:`；L399 `case parallelTaskFailed:`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function formatParallelTasksAggregate（internal/agent/parallel_tasks.go:368-406）
- **可信生产调用方（1）：** `ParallelTasksTool::Execute`（`internal/agent/parallel_tasks.go:111`，调用点 L345/L354/L356，confidence=0.90，exact-match，3 条边）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `ParallelTasksTool::Execute`（`internal/agent/parallel_tasks.go:111`，调用点 L345，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/parallel_tasks.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `formatParallelTasksAggregate` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L371、L382、L385、L393、L399 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `formatParallelTasksAggregate` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/parallel_tasks.go|formatParallelTasksAggregate` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/parallel_tasks.go:368-406` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `formatParallelTasksAggregate` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/parallel_tasks.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/parallel_tasks.go|formatParallelTasksAggregate` 下降或消失且无新增/恶化。
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
