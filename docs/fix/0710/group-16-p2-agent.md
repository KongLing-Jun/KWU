# G16 · Agent 主循环与工具执行链路

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G16 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/agent` |
| 涉及文件 | `internal/agent/agent.go`<br>`internal/agent/textsink.go`<br>`internal/agent/branch.go`<br>`internal/agent/save.go` |
| 主要治理目标 | 围绕 `internal/agent/agent.go` 等治理对象降低复杂度/体量，统一处理 1 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | agent 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614`、`origin/expert-202607081622`；单文件最多集中 11 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 16 组；等待 G09 的同文件变更合并 |
| 前置依赖 | G09（internal/agent/agent.go） |
| 后续验证重点 | 并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；返回值、错误语义和现有调用顺序；配置、平台或向后兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **51/100**，对象 `internal/agent/agent.go` 的分项为严重度 12.6、集中度 8.2、影响面 26.0、可执行性 4.0；最高超限 2.04x，文件内 11 条问题，CodeGraph 记录 3 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 4 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G09。后续同文件组：G31，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `FL-002` | `file_lines\|internal/agent/agent.go\|` | 文件有效行数 | `internal/agent/agent.go` | `internal/agent/agent.go` | 1 | 1632/800 | +832（2.04x） | P2 | 3 可信 + 0 待核 caller / 30 可信 + 7 待核 callee / 0 unresolved | 待修改 |
| `CX-057` | `complexity\|internal/agent/textsink.go\|*TextSink.Emit` | 圈复杂度 | `internal/agent/textsink.go` | `*TextSink.Emit` | 51 | 25/10 | +15（2.50x） | P2 | 0 可信 + 0 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-022` | `complexity\|internal/agent/agent.go\|*Agent.stream` | 圈复杂度 | `internal/agent/agent.go` | `*Agent.stream` | 1363 | 25/10 | +15（2.50x） | P2 | 0 可信 + 1 待核 caller / 3 可信 + 5 待核 callee / 0 unresolved | 待修改 |
| `FN-003` | `func_lines\|internal/agent/agent.go\|*Agent.stream` | 函数有效行数 | `internal/agent/agent.go` | `*Agent.stream` | 1363 | 92/80 | +12（1.15x） | P2 | 0 可信 + 1 待核 caller / 3 可信 + 5 待核 callee / 0 unresolved | 待修改 |
| `CX-026` | `complexity\|internal/agent/branch.go\|saveBranchMeta` | 圈复杂度 | `internal/agent/branch.go` | `saveBranchMeta` | 121 | 12/10 | +2（1.20x） | P2 | 5 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-047` | `complexity\|internal/agent/save.go\|ListSessionOrder` | 圈复杂度 | `internal/agent/save.go` | `ListSessionOrder` | 339 | 13/10 | +3（1.30x） | P2 | 4 可信 + 0 待核 caller / 2 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-027` | `complexity\|internal/agent/branch.go\|ListBranches` | 圈复杂度 | `internal/agent/branch.go` | `ListBranches` | 193 | 13/10 | +3（1.30x） | P2 | 1 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-058` | `complexity\|internal/agent/textsink.go\|FormatUsageLine` | 圈复杂度 | `internal/agent/textsink.go` | `FormatUsageLine` | 185 | 11/10 | +1（1.10x） | P3 | 2 可信 + 0 待核 caller / 0 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-020` | `complexity\|internal/agent/agent.go\|*Agent.finalReadinessCheck` | 圈复杂度 | `internal/agent/agent.go` | `*Agent.finalReadinessCheck` | 932 | 17/10 | +7（1.70x） | P3 | 0 可信 + 2 待核 caller / 8 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-021` | `complexity\|internal/agent/agent.go\|*Agent.rebuildTodoState` | 圈复杂度 | `internal/agent/agent.go` | `*Agent.rebuildTodoState` | 1119 | 12/10 | +2（1.20x） | P3 | 0 可信 + 2 待核 caller / 5 可信 + 1 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### FL-002 · 文件有效行数 · `internal/agent/agent.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-002 |
| Baseline key | `file_lines\|internal/agent/agent.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 51） |
| 文件 | `internal/agent/agent.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/agent/agent.go` |
| Baseline / 当前复测 / 阈值 | 1632 / 1632 / 800 |
| 超出 | +832，2.04x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/agent/agent.go` 当前有效代码行 **1632**，超过限制 **800**（+832）。CodeGraph 在文件中索引到的主要大符号包括 Agent::Run@675-879、Agent::executeOneWithRegistry@1826-1981、Agent::executeBatchWithRegistry@1504-1627、Agent::stream@1363-1466、New@603-659、Agent::finalReadinessCheck@932-978；文件同时承载的调用影响覆盖 19 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 110 个函数/方法节点。
- **可信生产调用方（3）：** `Build`（`internal/boot/boot.go:152`，调用点 L1038/L774/L775，confidence=0.90，import，3 条边）；`AskTool::Execute`（`internal/agent/ask.go:71`，调用点 L118/L125，confidence=0.90，exact-match，2 条边）；`Coordinator::persistExecutorNoOp`（`internal/agent/coordinator.go:252`，调用点 L256，confidence=0.90，exact-match，2 条边）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（30）：** `ReceiptFromToolCall`（`internal/evidence/evidence.go:610`，调用点 L1942/L1948/L1128/L1144，confidence=0.90，import，5 条边）；`IsNil`（`internal/nilutil/nil.go:6`，调用点 L616/L620/L624/L390，confidence=0.90，import，4 条边）；`RecordReadinessAudit`（`internal/event/event.go:347`，调用点 L800/L803/L827，confidence=0.90，import，3 条边）；`Ledger::Record`（`internal/evidence/evidence.go:70`，调用点 L1943/L1949/L1067，confidence=0.90，exact-match，3 条边）；`NewRegistry`（`internal/tool/tool.go:136`，调用点 L1828/L1506/L1638，confidence=0.90，import，3 条边）；`MatchStep`（`internal/evidence/evidence.go:310`，调用点 L1145/L1041，confidence=0.90，import，2 条边）；`PreviewChange`（`internal/tool/tool.go:48`，调用点 L1513/L1650，confidence=0.90，import，2 条边）；`CompareShape`（`internal/agent/cache_shape.go:67`，调用点 L766，confidence=0.90，exact-match）
- **待源码复核被调候选（7）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L858/L859/L1539/L1551，confidence=0.70，instance-method，11 条边，待源码复核）；`Agent::maybeCompact`（`internal/agent/compact.go:84`，调用点 L806/L816/L823/L835，confidence=0.70，instance-method，5 条边，待源码复核）；`Registry::Get`（`internal/tool/tool.go:233`，调用点 L1830/L1509/L1590/L1609，confidence=0.80，instance-method，5 条边，待源码复核）；`Registry::Len`（`internal/tool/tool.go:242`，调用点 L1415/L1485，confidence=0.70，instance-method，2 条边，待源码复核）；`AskTool::Name`（`internal/agent/ask.go:25`，调用点 L814，confidence=0.70，exact-match，待源码复核）；`NormalizeReasoningLanguage`（`internal/agent/reasoning_language.go:14`，调用点 L352，confidence=0.70，exact-match，待源码复核）；`MockProvider::Reset`（`internal/agent/testutil/mock_provider.go:177`，调用点 L681，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 19 个文件；`internal/agent`、`internal/boot`、`internal/automation`、`internal/evidence`、`internal/tool`、`internal/nilutil`、`internal/event`、`internal/instruction`、`internal/jobs`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/planmode_test.go`、`internal/agent/storm_test.go`、`internal/control/goal_test.go`、`internal/agent/compact_test.go`、`internal/agent/outcome_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/gate_test.go`、`internal/agent/hooks_test.go`、`desktop/history_test.go`、`internal/agent/evidence_flow_test.go`、`internal/agent/expert_team_tools_test.go`、`internal/agent/final_readiness_test.go`、`internal/agent/guards_test.go`、`internal/agent/nil_boundary_test.go`、`internal/agent/steer_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/agent.go`、`internal/boot/boot.go`、`internal/agent/ask.go`、`internal/agent/coordinator.go`、`internal/automation/archive_sink.go`、`internal/agent/compact.go`、`internal/evidence/evidence.go`、`internal/tool/tool.go`、`internal/agent/planmode_test.go`、`internal/agent/storm_test.go`、`internal/control/goal_test.go`、`internal/agent/compact_test.go`

#### 待修改项

- [ ] 先冻结 `internal/agent/agent.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `Agent::Run`、`Agent::executeOneWithRegistry`、`Agent::executeBatchWithRegistry`、`Agent::stream`、`New` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-019、CX-020、CX-021、CX-022、CX-023、CX-024、FN-002、FN-003、FN-004、FN-005。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/agent/agent.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/agent.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/agent/agent.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/planmode_test.go`、`internal/agent/storm_test.go`、`internal/control/goal_test.go`、`internal/agent/compact_test.go`、`internal/agent/outcome_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/agent.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/agent/agent.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-057 · 圈复杂度 · `*TextSink.Emit`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-057 |
| Baseline key | `complexity\|internal/agent/textsink.go\|*TextSink.Emit` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 49） |
| 文件 | `internal/agent/textsink.go` |
| 代码行 | 51 |
| 函数/方法/包 | `*TextSink.Emit` |
| Baseline / 当前复测 / 阈值 | 25 / 25 / 10 |
| 超出 | +15，2.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*TextSink.Emit` 的圈复杂度当前为 **25**，超过限制 **10**（+15，2.50x）。定义范围为 51-137 行；可见控制流热点：L52 `switch e.Kind {`；L64 `if s.showReasoning && e.Text != "" {`；L81 `case event.ToolDispatch:`；L101 `if s.textWritten {`；L116 `if s.wroteAnything {`；L132 `for _, ln := range strings.Split(strings.TrimRight(c.Summary, "\n"), "\n") {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（TextSink::closeTextStream）、持久化/I-O（TextSink::closeTextStream）、网络/协议（TextSink::closeTextStream）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method TextSink::Emit（internal/agent/textsink.go:51-137）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `dimText`（`internal/agent/textsink.go:222`，调用点 L61/L65/L123/L131，confidence=0.90，exact-match，5 条边）；`CompactArgs`（`internal/agent/textsink.go:226`，调用点 L87，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `TextSink::closeTextStream`（`internal/agent/textsink.go:144`，调用点 L79，confidence=0.70，instance-method，待源码复核）；`TextSink::usageLine`（`internal/agent/textsink.go:170`，调用点 L105，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/usage_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/textsink.go`、`internal/agent/usage_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`

#### 待修改项

- [ ] 先冻结 `*TextSink.Emit` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L52、L70、L93、L115、L132 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*TextSink.Emit` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/textsink.go|*TextSink.Emit` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/textsink.go:51-137` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*TextSink.Emit` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/usage_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/textsink.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/textsink.go|*TextSink.Emit` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-022 · 圈复杂度 · `*Agent.stream`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-022 |
| Baseline key | `complexity\|internal/agent/agent.go\|*Agent.stream` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 48） |
| 文件 | `internal/agent/agent.go` |
| 代码行 | 1363 |
| 函数/方法/包 | `*Agent.stream` |
| Baseline / 当前复测 / 阈值 | 25 / 25 / 10 |
| 超出 | +15，2.50x |
| 同一治理对象关联问题 | `FN-003` |

#### 明确的问题描述

`*Agent.stream` 的圈复杂度当前为 **25**，超过限制 **10**（+15，2.50x）。定义范围为 1363-1466 行；可见控制流热点：L1372 `if err != nil {`；L1402 `for {`；L1409 `if !ok {`；L1430 `if chunk.Signature != "" {`；L1445 `if tc := chunk.ToolCall; tc != nil {`；L1459 `if provider.IsStreamInterrupted(chunk.Err) {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（ArchiveSink::Err）、网络/协议（IsStreamInterrupted）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::stream（internal/agent/agent.go:1363-1466）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L742，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `StripGoalMarkers`（`internal/agent/goal_display.go:12`，调用点 L1418，confidence=0.90，exact-match）；`IsStreamInterrupted`（`internal/provider/provider.go:584`，调用点 L1459，confidence=0.90，import）；`WithRetryNotify`（`internal/provider/retry.go:55`，调用点 L1364，confidence=0.90，import）
- **待源码复核被调候选（5）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L1407/L1410，confidence=0.70，instance-method，2 条边，待源码复核）；`ToolHooks::PostLLMCall`（`internal/agent/agent.go:153`，调用点 L1391，confidence=0.70，exact-match，待源码复核）；`ToolHooks::HasPostLLMCall`（`internal/agent/agent.go:154`，调用点 L1380，confidence=0.70，exact-match，待源码复核）；`Agent::memoryCitations`（`internal/agent/agent.go:1468`，调用点 L1420，confidence=0.70，instance-method，待源码复核）；`Registry::Len`（`internal/tool/tool.go:242`，调用点 L1415，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L742，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/agent`、`internal/provider`、`internal/automation`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/agent.go`、`internal/agent/goal_display.go`、`internal/provider/provider.go`、`internal/provider/retry.go`、`internal/automation/archive_sink.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.stream` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1372、L1404、L1427、L1439、L1459 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Agent.stream` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-022、FN-003 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/agent.go|*Agent.stream` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/agent.go:1363-1466` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.stream` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-022`、`FN-003` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/agent.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/agent.go|*Agent.stream` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-003 · 函数有效行数 · `*Agent.stream`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-003 |
| Baseline key | `func_lines\|internal/agent/agent.go\|*Agent.stream` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 48） |
| 文件 | `internal/agent/agent.go` |
| 代码行 | 1363 |
| 函数/方法/包 | `*Agent.stream` |
| Baseline / 当前复测 / 阈值 | 92 / 92 / 80 |
| 超出 | +12，1.15x |
| 同一治理对象关联问题 | `CX-022` |

#### 明确的问题描述

`*Agent.stream` 的有效代码行 **92**，超过限制 **80**（+12）。CodeGraph 定义物理范围为 1363-1466 行，并记录 8 个直接下游候选；候选职责分布为 持久化/I-O（ArchiveSink::Err）、网络/协议（IsStreamInterrupted）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::stream（internal/agent/agent.go:1363-1466）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L742，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `StripGoalMarkers`（`internal/agent/goal_display.go:12`，调用点 L1418，confidence=0.90，exact-match）；`IsStreamInterrupted`（`internal/provider/provider.go:584`，调用点 L1459，confidence=0.90，import）；`WithRetryNotify`（`internal/provider/retry.go:55`，调用点 L1364，confidence=0.90，import）
- **待源码复核被调候选（5）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L1407/L1410，confidence=0.70，instance-method，2 条边，待源码复核）；`ToolHooks::PostLLMCall`（`internal/agent/agent.go:153`，调用点 L1391，confidence=0.70，exact-match，待源码复核）；`ToolHooks::HasPostLLMCall`（`internal/agent/agent.go:154`，调用点 L1380，confidence=0.70，exact-match，待源码复核）；`Agent::memoryCitations`（`internal/agent/agent.go:1468`，调用点 L1420，confidence=0.70，instance-method，待源码复核）；`Registry::Len`（`internal/tool/tool.go:242`，调用点 L1415，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L742，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/agent`、`internal/provider`、`internal/automation`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/agent.go`、`internal/agent/goal_display.go`、`internal/provider/provider.go`、`internal/provider/retry.go`、`internal/automation/archive_sink.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.stream` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*Agent.stream` 的 92 个有效行按 持久化/I-O、网络/协议 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L1376 `A PostLLMCall hook rewrites the whole reasoning block, so when one is wired`、L1378 `stream. With no such hook the reasoning streams live, chunk by chunk, as`、L1442 `(possibly large) arguments finish streaming — so the user sees it`、L1444 `(with args) once the call completes; the frontend merges by ID.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-022、FN-003 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/agent/agent.go|*Agent.stream` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/agent.go:1363-1466` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.stream` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-022`、`FN-003` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/agent.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/agent/agent.go|*Agent.stream` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-026 · 圈复杂度 · `saveBranchMeta`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-026 |
| Baseline key | `complexity\|internal/agent/branch.go\|saveBranchMeta` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 47） |
| 文件 | `internal/agent/branch.go` |
| 代码行 | 121 |
| 函数/方法/包 | `saveBranchMeta` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`saveBranchMeta` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 121-163 行；可见控制流热点：L123 `if metaPath == "" {`；L130 `if m.CreatedAt.IsZero() {`；L136 `if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {`；L140 `if err != nil {`；L149 `if _, err := tmp.Write(b); err != nil {`；L158 `if err := fileutil.ReplaceFile(tmpPath, metaPath); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（ReplaceFile）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function saveBranchMeta（internal/agent/branch.go:121-163）
- **可信生产调用方（5）：** `SaveBranchMeta`（`internal/agent/branch.go:113`，调用点 L114，confidence=0.90，exact-match）；`SaveBranchMetaPreserveUpdated`（`internal/agent/branch.go:117`，调用点 L118，confidence=0.90，exact-match）；`EnsureBranchMeta`（`internal/agent/branch.go:165`，调用点 L181，confidence=0.90，exact-match）；`TouchBranchMeta`（`internal/agent/branch.go:184`，调用点 L190，confidence=0.90，exact-match）；`UpdateSessionMeta`（`internal/agent/branch.go:297`，调用点 L313，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `BranchID`（`internal/agent/branch.go:76`，调用点 L128，confidence=0.90，exact-match）；`BranchMetaPath`（`internal/agent/branch.go:87`，调用点 L122，confidence=0.90，exact-match）；`ReplaceFile`（`internal/fileutil/atomicwrite.go:67`，调用点 L158，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::Fork`（`desktop/app.go:1615`，调用点 L1656，confidence=0.90，import，待源码复核）；`TouchBranchMeta`（`internal/agent/branch.go:184`，调用点 L190，confidence=0.90，exact-match，待源码复核）；`SetBranchModelPreserveUpdated`（`internal/agent/branch.go:280`，调用点 L289，confidence=0.90，exact-match，待源码复核）；`Controller::Branch`（`internal/control/controller.go:2250`，调用点 L2281，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/agent`、`internal/fileutil`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/branch.go`、`internal/fileutil/atomicwrite.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `saveBranchMeta` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L123、L130、L140、L149、L158 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `saveBranchMeta` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/branch.go|saveBranchMeta` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/branch.go:121-163` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `saveBranchMeta` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/branch.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/branch.go|saveBranchMeta` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-047 · 圈复杂度 · `ListSessionOrder`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-047 |
| Baseline key | `complexity\|internal/agent/save.go\|ListSessionOrder` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 46） |
| 文件 | `internal/agent/save.go` |
| 代码行 | 339 |
| 函数/方法/包 | `ListSessionOrder` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ListSessionOrder` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 339-405 行；可见控制流热点：L341 `if err != nil {`；L348 `for _, e := range entries {`；L353 `if err != nil {`；L357 `if !IsVisibleSession(full) {`；L370 `if !meta.CreatedAt.IsZero() {`；L399 `if out[i].LastActivityAt.Equal(out[j].LastActivityAt) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（LoadBranchMeta）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ListSessionOrder（internal/agent/save.go:339-405）
- **可信生产调用方（4）：** `promptHistorySessionFiles`（`desktop/app.go:2996`，调用点 L2997，confidence=0.90，import）；`migrateLegacySessionsIntoGlobalTopics`（`desktop/tabs.go:3931`，调用点 L3951，confidence=0.90，import）；`ListSessions`（`internal/agent/save.go:411`，调用点 L412，confidence=0.90，exact-match）；`listSessionsTool::Execute`（`internal/tool/sessiontool/sessiontool.go:41`，调用点 L42，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `LoadBranchMeta`（`internal/agent/branch.go:91`，调用点 L369，confidence=0.90，exact-match）；`IsVisibleSession`（`internal/agent/save.go:267`，调用点 L357，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `BranchMeta::DefaultScope`（`internal/agent/branch.go:57`，调用点 L376，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `App::ListSessions`（`desktop/app.go:1888`，调用点 L1890，confidence=0.90，import，待源码复核）；`App::ListTrashedSessions`（`desktop/app.go:1912`，调用点 L1921，confidence=0.90，import，待源码复核）；`App::ListProjectTree`（`desktop/tabs.go:4819`，调用点 L4876，confidence=0.90，import，待源码复核）；`chatREPL`（`internal/cli/cli.go:652`，调用点 L686，confidence=0.90，import，待源码复核）；`listSessionsTool::Execute`（`internal/tool/sessiontool/sessiontool.go:41`，调用点 L42，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/agent`、`desktop`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/listsessions_bench_test.go`、`internal/agent/save_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/save.go`、`desktop/app.go`、`desktop/tabs.go`、`internal/tool/sessiontool/sessiontool.go`、`internal/agent/branch.go`、`internal/agent/listsessions_bench_test.go`、`internal/agent/save_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`

#### 待修改项

- [ ] 先冻结 `ListSessionOrder` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L341、L348、L357、L370、L399 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `ListSessionOrder` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/save.go|ListSessionOrder` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/save.go:339-405` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ListSessionOrder` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/listsessions_bench_test.go`、`internal/agent/save_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/save.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/save.go|ListSessionOrder` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-027 · 圈复杂度 · `ListBranches`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-027 |
| Baseline key | `complexity\|internal/agent/branch.go\|ListBranches` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 44） |
| 文件 | `internal/agent/branch.go` |
| 代码行 | 193 |
| 函数/方法/包 | `ListBranches` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ListBranches` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 193-247 行；可见控制流热点：L195 `if err != nil {`；L202 `for _, e := range entries {`；L207 `if err != nil {`；L215 `if turns == 0 {`；L222 `if !ok {`；L241 `if out[i].CreatedAt.Equal(out[j].CreatedAt) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（LoadBranchMeta）、渲染/输出（previewSession）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ListBranches（internal/agent/branch.go:193-247）
- **可信生产调用方（1）：** `Controller::Branches`（`internal/control/controller.go:2310`，调用点 L2317，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `BranchID`（`internal/agent/branch.go:76`，调用点 L224/L230，confidence=0.90，exact-match，2 条边）；`LoadBranchMeta`（`internal/agent/branch.go:91`，调用点 L218，confidence=0.90，exact-match）；`IsVisibleSession`（`internal/agent/save.go:267`，调用点 L211，confidence=0.90，exact-match）；`previewSession`（`internal/agent/save.go:485`，调用点 L214，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::Branches`（`internal/control/controller.go:2310`，调用点 L2317，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/agent`、`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/branch_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/branch.go`、`internal/control/controller.go`、`internal/agent/save.go`、`internal/agent/branch_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `ListBranches` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L195、L203、L211、L222、L241 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `ListBranches` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/branch.go|ListBranches` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/branch.go:193-247` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ListBranches` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/branch_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/branch.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/branch.go|ListBranches` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-058 · 圈复杂度 · `FormatUsageLine`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-058 |
| Baseline key | `complexity\|internal/agent/textsink.go\|FormatUsageLine` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/agent/textsink.go` |
| 代码行 | 185 |
| 函数/方法/包 | `FormatUsageLine` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`FormatUsageLine` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 185-218 行；可见控制流热点：L186 `if u == nil || u.TotalTokens == 0 {`；L190 `if u.PromptTokens > 0 {`；L194 `if d := u.PromptTokens - cached; d > 0 {`；L201 `if u.ReasoningTokens > 0 {`；L209 `if d != nil && d.PrefixChanged {`；L211 `if reasons == "" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function FormatUsageLine（internal/agent/textsink.go:185-218）
- **可信生产调用方（2）：** `TextSink::usageLine`（`internal/agent/textsink.go:170`，调用点 L171，confidence=0.90，exact-match）；`chatTUI::ingestEvent`（`internal/cli/chat_tui.go:3004`，调用点 L3110，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（2）：** `Pricing::Cost`（`internal/provider/provider.go:481`，调用点 L206，confidence=0.70，instance-method，待源码复核）；`Pricing::Symbol`（`internal/provider/provider.go:498`，调用点 L206，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `TextSink::Emit`（`internal/agent/textsink.go:51`，调用点 L105，confidence=0.70，instance-method，待源码复核）；`chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1200，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/agent`、`internal/cli`、`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/cachehit_e2e_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/textsink.go`、`internal/cli/chat_tui.go`、`internal/provider/provider.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`

#### 待修改项

- [ ] 先冻结 `FormatUsageLine` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L186、L193、L201、L205、L211 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `FormatUsageLine` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/textsink.go|FormatUsageLine` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/textsink.go:185-218` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `FormatUsageLine` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/cachehit_e2e_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/textsink.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/textsink.go|FormatUsageLine` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-020 · 圈复杂度 · `*Agent.finalReadinessCheck`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-020 |
| Baseline key | `complexity\|internal/agent/agent.go\|*Agent.finalReadinessCheck` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 41） |
| 文件 | `internal/agent/agent.go` |
| 代码行 | 932 |
| 函数/方法/包 | `*Agent.finalReadinessCheck` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Agent.finalReadinessCheck` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 932-978 行；可见控制流热点：L933 `if a.evidence == nil {`；L940 `if !hasTodos && a.evidence.HasAnySuccessfulReceipt() {`；L950 `if !hasWriter {`；L958 `if !hasProjectChecks && !hasTodoReceipt && len(missing) == 0 {`；L964 `if command == "" {`；L973 `if len(missing) == 0 {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（finalReadinessIncompleteTodos、finalReadinessCheckSource、Ledger::HasSuccessfulTodoWrite、Ledger::LatestSuccessfulWriterIndex）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::finalReadinessCheck（internal/agent/agent.go:932-978）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L794，confidence=0.70，instance-method，待源码复核）；`Agent::finalReadinessFailure`（`internal/agent/agent.go:904`，调用点 L905，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（8）：** `finalReadinessIncompleteTodos`（`internal/agent/agent.go:980`，调用点 L946，confidence=0.90，exact-match）；`finalReadinessCheckSource`（`internal/agent/agent.go:1175`，调用点 L969，confidence=0.90，exact-match）；`Ledger::HasSuccessfulCommandAfter`（`internal/evidence/evidence.go:200`，调用点 L967，confidence=0.90，exact-match）；`Ledger::HasSuccessfulTodoWrite`（`internal/evidence/evidence.go:241`，调用点 L957，confidence=0.90，exact-match）；`Ledger::HasSuccessfulTodoProgressReceipt`（`internal/evidence/evidence.go:258`，调用点 L943，confidence=0.90，exact-match）；`Ledger::IncompleteLatestTodos`（`internal/evidence/evidence.go:273`，调用点 L939，confidence=0.90，exact-match）；`Ledger::HasAnySuccessfulReceipt`（`internal/evidence/evidence.go:317`，调用点 L940，confidence=0.90，exact-match）；`Ledger::LatestSuccessfulWriterIndex`（`internal/evidence/evidence.go:362`，调用点 L949，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Agent::incompleteCanonicalTodos`（`internal/agent/agent.go:1022`，调用点 L941，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L794，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/agent`、`internal/evidence`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/final_readiness_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/agent.go`、`internal/evidence/evidence.go`、`internal/agent/final_readiness_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.finalReadinessCheck` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L933、L943、L951、L964、L973 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Agent.finalReadinessCheck` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/agent.go|*Agent.finalReadinessCheck` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/agent.go:932-978` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.finalReadinessCheck` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/final_readiness_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/agent.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/agent.go|*Agent.finalReadinessCheck` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-021 · 圈复杂度 · `*Agent.rebuildTodoState`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-021 |
| Baseline key | `complexity\|internal/agent/agent.go\|*Agent.rebuildTodoState` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/agent/agent.go` |
| 代码行 | 1119 |
| 函数/方法/包 | `*Agent.rebuildTodoState` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Agent.rebuildTodoState` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 1119-1152 行；可见控制流热点：L1123 `for i, msg := range msgs {`；L1124 `for _, tc := range msg.ToolCalls {`；L1135 `if baseIdx < 0 {`；L1139 `for i := baseIdx; i < len(msgs); i++ {`；L1141 `if tc.Name != "complete_step" || !successful[tc.ID] {`；L1145 `if m, ok := evidence.MatchStep(rec.Step, todos); ok && canonicalTodoStatus(todos[m.Index-1].Status)…`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（Agent::setTodoState）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::rebuildTodoState（internal/agent/agent.go:1119-1152）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `Agent::SetSession`（`internal/agent/agent.go:444`，调用点 L451，confidence=0.70，instance-method，待源码复核）；`Agent::RebuildTodoState`（`internal/agent/agent.go:1110`，调用点 L1111，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（5）：** `ReceiptFromToolCall`（`internal/evidence/evidence.go:610`，调用点 L1128/L1144，confidence=0.90，import，2 条边）；`promoteNextPendingTodo`（`internal/agent/agent.go:1070`，调用点 L1147，confidence=0.90，exact-match）；`canonicalTodoStatus`（`internal/agent/agent.go:1084`，调用点 L1145，confidence=0.90，exact-match）；`successfulToolCallIDs`（`internal/agent/agent.go:1154`，调用点 L1120，confidence=0.90，exact-match）；`MatchStep`（`internal/evidence/evidence.go:310`，调用点 L1145，confidence=0.90，import）
- **待源码复核被调候选（1）：** `Agent::setTodoState`（`internal/agent/agent.go:992`，调用点 L1136/L1151，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** `Coordinator::ResetPlannerSession`（`internal/agent/coordinator.go:113`，调用点 L124，confidence=0.90，exact-match，待源码复核）；`Controller::SetExpert`（`internal/control/controller.go:1801`，调用点 L1825，confidence=0.90，exact-match，待源码复核）；`Controller::ClearExpert`（`internal/control/controller.go:1832`，调用点 L1852，confidence=0.90，exact-match，待源码复核）；`Controller::NewSession`（`internal/control/controller.go:1946`，调用点 L1961，confidence=0.90，exact-match，待源码复核）；`Controller::ClearSession`（`internal/control/controller.go:1977`，调用点 L2010，confidence=0.90，exact-match，待源码复核）；`Controller::Branch`（`internal/control/controller.go:2250`，调用点 L2293，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/agent`、`internal/evidence`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/canonical_todo_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/agent.go`、`internal/evidence/evidence.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.rebuildTodoState` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1123、L1125、L1139、L1140、L1145 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Agent.rebuildTodoState` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/agent.go|*Agent.rebuildTodoState` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/agent.go:1119-1152` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.rebuildTodoState` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/canonical_todo_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/agent.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/agent.go|*Agent.rebuildTodoState` 下降或消失且无新增/恶化。
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
