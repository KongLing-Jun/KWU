# G09 · Agent 主循环与工具执行链路

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G09 |
| 问题数量 | 10 |
| 整体优先级 | P1 |
| 涉及模块 | `internal/agent` |
| 涉及文件 | `internal/agent/task.go`<br>`internal/agent/agent.go` |
| 主要治理目标 | 围绕 `internal/agent/agent.go` 等治理对象降低复杂度/体量，统一处理 4 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | agent 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614`、`origin/expert-202607081622`；单文件最多集中 11 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 9 组；先于 G16 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 持久化、迁移或文件格式兼容；会话、状态或恢复语义；并发、取消或事件顺序；返回值、错误语义和现有调用顺序；协议、序列化或流式事件兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **59/100**，对象 `*TaskTool.Execute` 的分项为严重度 10.7、集中度 10.3、影响面 21.8、可执行性 16.0；最高超限 2.50x，文件内 4 条问题，CodeGraph 记录 0 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数，必须合并治理。

### 治理边界与组间关系

本组涉及 2 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G16，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-053` | `complexity\|internal/agent/task.go\|*TaskTool.Execute` | 圈复杂度 | `internal/agent/task.go` | `*TaskTool.Execute` | 434 | 25/10 | +15（2.50x） | P1 | 0 可信 + 0 待核 caller / 12 可信 + 6 待核 callee / 0 unresolved | 待修改 |
| `FN-009` | `func_lines\|internal/agent/task.go\|*TaskTool.Execute` | 函数有效行数 | `internal/agent/task.go` | `*TaskTool.Execute` | 434 | 93/80 | +13（1.16x） | P1 | 0 可信 + 0 待核 caller / 12 可信 + 6 待核 callee / 0 unresolved | 待修改 |
| `CX-024` | `complexity\|internal/agent/agent.go\|*Agent.executeOneWithRegistry` | 圈复杂度 | `internal/agent/agent.go` | `*Agent.executeOneWithRegistry` | 1826 | 38/10 | +28（3.80x） | P2 | 0 可信 + 2 待核 caller / 14 可信 + 11 待核 callee / 0 unresolved | 待修改 |
| `FN-005` | `func_lines\|internal/agent/agent.go\|*Agent.executeOneWithRegistry` | 函数有效行数 | `internal/agent/agent.go` | `*Agent.executeOneWithRegistry` | 1826 | 135/80 | +55（1.69x） | P2 | 0 可信 + 2 待核 caller / 14 可信 + 11 待核 callee / 0 unresolved | 待修改 |
| `CX-019` | `complexity\|internal/agent/agent.go\|*Agent.Run` | 圈复杂度 | `internal/agent/agent.go` | `*Agent.Run` | 675 | 38/10 | +28（3.80x） | P2 | 0 可信 + 0 待核 caller / 14 可信 + 20 待核 callee / 0 unresolved | 待修改 |
| `FN-002` | `func_lines\|internal/agent/agent.go\|*Agent.Run` | 函数有效行数 | `internal/agent/agent.go` | `*Agent.Run` | 675 | 172/80 | +92（2.15x） | P2 | 0 可信 + 0 待核 caller / 14 可信 + 20 待核 callee / 0 unresolved | 待修改 |
| `CX-023` | `complexity\|internal/agent/agent.go\|*Agent.executeBatchWithRegistry` | 圈复杂度 | `internal/agent/agent.go` | `*Agent.executeBatchWithRegistry` | 1504 | 30/10 | +20（3.00x） | P2 | 0 可信 + 2 待核 caller / 5 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `FN-004` | `func_lines\|internal/agent/agent.go\|*Agent.executeBatchWithRegistry` | 函数有效行数 | `internal/agent/agent.go` | `*Agent.executeBatchWithRegistry` | 1504 | 110/80 | +30（1.38x） | P2 | 0 可信 + 2 待核 caller / 5 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `CX-055` | `complexity\|internal/agent/task.go\|subSinkForContext` | 圈复杂度 | `internal/agent/task.go` | `subSinkForContext` | 869 | 13/10 | +3（1.30x） | P2 | 5 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-054` | `complexity\|internal/agent/task.go\|ReadOnlySubagentToolRegistry` | 圈复杂度 | `internal/agent/task.go` | `ReadOnlySubagentToolRegistry` | 672 | 11/10 | +1（1.10x） | P2 | 2 可信 + 0 待核 caller / 2 可信 + 1 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-053 · 圈复杂度 · `*TaskTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-053 |
| Baseline key | `complexity\|internal/agent/task.go\|*TaskTool.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P1，治理对象得分 59） |
| 文件 | `internal/agent/task.go` |
| 代码行 | 434 |
| 函数/方法/包 | `*TaskTool.Execute` |
| Baseline / 当前复测 / 阈值 | 25 / 25 / 10 |
| 超出 | +15，2.50x |
| 同一治理对象关联问题 | `FN-009` |

#### 明确的问题描述

`*TaskTool.Execute` 的圈复杂度当前为 **25**，超过限制 **10**（+15，2.50x）。定义范围为 434-542 行；可见控制流热点：L446 `if err := json.Unmarshal(args, &p); err != nil {`；L462 `if maxSteps < 5 {`；L487 `if !ok {`；L499 `if t.transcripts != nil && run != nil && run.Ref != "" {`；L518 `if err := t.transcripts.SaveCompleted(run); err != nil {`；L536 `if err := t.transcripts.SaveCompleted(run); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（TaskTool::resolveSubSessionRuntime）、状态与控制（FormatSubagentRunResult、SubagentStore::MarkRunning、TaskTool::runSubSession、TaskTool::prepareTranscriptRun）、持久化/I-O（SubagentStore::SaveFailed、SubagentStore::SaveCompleted、SubagentStore::MarkRunning、TaskTool::effectiveProfile）、渲染/输出（FormatSubagentRunResult、FormatSubagentReference）、任务/并发（TaskTreeContextFromContext、TaskTool::runSubSession、TaskTool::effectiveProfile、TaskTool::prepareTranscriptRun）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method TaskTool::Execute（internal/agent/task.go:434-542）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（12）：** `SubagentStore::SaveFailed`（`internal/agent/subagent_store.go:553`，调用点 L511/L516/L519/L533，confidence=0.90，exact-match，5 条边）；`FormatSubagentRunResult`（`internal/agent/task.go:788`，调用点 L510/L516/L519/L521，confidence=0.90，exact-match，5 条边）；`SubagentStore::SaveCompleted`（`internal/agent/subagent_store.go:536`，调用点 L518/L536，confidence=0.90，exact-match，2 条边）；`CallContext`（`internal/agent/agent.go:88`，调用点 L470，confidence=0.90，exact-match）；`ParentSession`（`internal/agent/agent.go:111`，调用点 L471，confidence=0.90，exact-match）；`SubagentStore::MarkRunning`（`internal/agent/subagent_store.go:523`，调用点 L500，confidence=0.90，exact-match）；`FormatSubagentReference`（`internal/agent/task.go:771`，调用点 L524，confidence=0.90，exact-match）；`subSink`（`internal/agent/task.go:853`，调用点 L531，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `TaskTool::runSubSession`（`internal/agent/task.go:752`，调用点 L514/L531，confidence=0.70，instance-method，2 条边，待源码复核）；`TaskTool::effectiveProfile`（`internal/agent/task.go:422`，调用点 L469，confidence=0.70，instance-method，待源码复核）；`TaskTool::prepareTranscriptRun`（`internal/agent/task.go:544`，调用点 L471，confidence=0.70，instance-method，待源码复核）；`TaskTool::buildSubReg`（`internal/agent/task.go:618`，调用点 L468，confidence=0.70，instance-method，待源码复核）；`TaskTool::resolveSubSessionRuntime`（`internal/agent/task.go:740`，调用点 L475，confidence=0.70，instance-method，待源码复核）；`Manager::StartForSession`（`internal/jobs/jobs.go:242`，调用点 L505，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/agent`、`internal/event`、`internal/jobs`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/task.go`、`internal/agent/subagent_store.go`、`internal/agent/agent.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*TaskTool.Execute` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L446、L472、L496、L515、L536 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*TaskTool.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-053、FN-009 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/task.go|*TaskTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/task.go:434-542` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*TaskTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G16 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-053`、`FN-009` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/task.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/task.go|*TaskTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-009 · 函数有效行数 · `*TaskTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-009 |
| Baseline key | `func_lines\|internal/agent/task.go\|*TaskTool.Execute` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P3，治理对象得分 59） |
| 文件 | `internal/agent/task.go` |
| 代码行 | 434 |
| 函数/方法/包 | `*TaskTool.Execute` |
| Baseline / 当前复测 / 阈值 | 93 / 93 / 80 |
| 超出 | +13，1.16x |
| 同一治理对象关联问题 | `CX-053` |

#### 明确的问题描述

`*TaskTool.Execute` 的有效代码行 **93**，超过限制 **80**（+13）。CodeGraph 定义物理范围为 434-542 行，并记录 18 个直接下游候选；候选职责分布为 配置/解析（TaskTool::resolveSubSessionRuntime）、状态与控制（FormatSubagentRunResult、SubagentStore::MarkRunning、TaskTool::runSubSession、TaskTool::prepareTranscriptRun）、持久化/I-O（SubagentStore::SaveFailed、SubagentStore::SaveCompleted、SubagentStore::MarkRunning、TaskTool::effectiveProfile）、渲染/输出（FormatSubagentRunResult、FormatSubagentReference）、任务/并发（TaskTreeContextFromContext、TaskTool::runSubSession、TaskTool::effectiveProfile、TaskTool::prepareTranscriptRun）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method TaskTool::Execute（internal/agent/task.go:434-542）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（12）：** `SubagentStore::SaveFailed`（`internal/agent/subagent_store.go:553`，调用点 L511/L516/L519/L533，confidence=0.90，exact-match，5 条边）；`FormatSubagentRunResult`（`internal/agent/task.go:788`，调用点 L510/L516/L519/L521，confidence=0.90，exact-match，5 条边）；`SubagentStore::SaveCompleted`（`internal/agent/subagent_store.go:536`，调用点 L518/L536，confidence=0.90，exact-match，2 条边）；`CallContext`（`internal/agent/agent.go:88`，调用点 L470，confidence=0.90，exact-match）；`ParentSession`（`internal/agent/agent.go:111`，调用点 L471，confidence=0.90，exact-match）；`SubagentStore::MarkRunning`（`internal/agent/subagent_store.go:523`，调用点 L500，confidence=0.90，exact-match）；`FormatSubagentReference`（`internal/agent/task.go:771`，调用点 L524，confidence=0.90，exact-match）；`subSink`（`internal/agent/task.go:853`，调用点 L531，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `TaskTool::runSubSession`（`internal/agent/task.go:752`，调用点 L514/L531，confidence=0.70，instance-method，2 条边，待源码复核）；`TaskTool::effectiveProfile`（`internal/agent/task.go:422`，调用点 L469，confidence=0.70，instance-method，待源码复核）；`TaskTool::prepareTranscriptRun`（`internal/agent/task.go:544`，调用点 L471，confidence=0.70，instance-method，待源码复核）；`TaskTool::buildSubReg`（`internal/agent/task.go:618`，调用点 L468，confidence=0.70，instance-method，待源码复核）；`TaskTool::resolveSubSessionRuntime`（`internal/agent/task.go:740`，调用点 L475，confidence=0.70，instance-method，待源码复核）；`Manager::StartForSession`（`internal/jobs/jobs.go:242`，调用点 L505，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/agent`、`internal/event`、`internal/jobs`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/task.go`、`internal/agent/subagent_store.go`、`internal/agent/agent.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*TaskTool.Execute` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*TaskTool.Execute` 的 93 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L455 `No explicit cap from the caller: mirror the parent. A finite parent caps`、L458 `sub-agent. The sub-agent shares the parent's ctx, so cancelling the turn`、L482 `session context (so it survives this turn) and return immediately. The`、L529 `Foreground: run synchronously, nesting events under this call.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-053、FN-009 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/agent/task.go|*TaskTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/task.go:434-542` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*TaskTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G16 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-053`、`FN-009` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/task.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/agent/task.go|*TaskTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-024 · 圈复杂度 · `*Agent.executeOneWithRegistry`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-024 |
| Baseline key | `complexity\|internal/agent/agent.go\|*Agent.executeOneWithRegistry` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 56） |
| 文件 | `internal/agent/agent.go` |
| 代码行 | 1826 |
| 函数/方法/包 | `*Agent.executeOneWithRegistry` |
| Baseline / 当前复测 / 阈值 | 38 / 38 / 10 |
| 超出 | +28，3.80x |
| 同一治理对象关联问题 | `FN-005` |

#### 明确的问题描述

`*Agent.executeOneWithRegistry` 的圈复杂度当前为 **38**，超过限制 **10**（+28，3.80x）。定义范围为 1826-1981 行；可见控制流热点：L1827 `if registry == nil {`；L1858 `if u, ok := t.(tool.PlanModeUntrustedReadOnly); ok {`；L1890 `if msg == "" {`；L1919 `if a.jobs != nil {`；L1941 `if call.Name == "complete_step" {`；L1976 `if a.hooks != nil && call.Name == "task" && !isBackgroundTaskCall(call.Arguments) {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（truncateToolOutput、ToolHooks::SubagentStop、Agent::setTodoState）、网络/协议（WithResponseLanguagePreference）、任务/并发（isBackgroundTaskCall、WithQueue、Agent::planModeBlocked、Agent::repeatedSuccessBlock）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::executeOneWithRegistry（internal/agent/agent.go:1826-1981）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `Agent::executeBatchWithRegistry`（`internal/agent/agent.go:1504`，调用点 L1532，confidence=0.70，instance-method，待源码复核）；`Agent::executeOne`（`internal/agent/agent.go:1822`，调用点 L1823，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（14）：** `truncateToolOutput`（`internal/agent/agent.go:2157`，调用点 L1969/L1979，confidence=0.90，exact-match，2 条边）；`Ledger::Record`（`internal/evidence/evidence.go:70`，调用点 L1943/L1949，confidence=0.90，exact-match，2 条边）；`ReceiptFromToolCall`（`internal/evidence/evidence.go:610`，调用点 L1942/L1948，confidence=0.90，import，2 条边）；`withCallContext`（`internal/agent/agent.go:74`，调用点 L1911，confidence=0.90，exact-match）；`isBackgroundTaskCall`（`internal/agent/agent.go:2129`，调用点 L1976，confidence=0.90，exact-match）；`WithResponseLanguagePreference`（`internal/agent/reasoning_language.go:140`，调用点 L1924，confidence=0.90，exact-match）；`WithReasoningLanguagePreference`（`internal/agent/reasoning_language.go:162`，调用点 L1929，confidence=0.90，exact-match）；`WithLedger`（`internal/evidence/evidence.go:537`，调用点 L1913，confidence=0.90，import）
- **待源码复核被调候选（11）：** `Gate::Check`（`internal/agent/agent.go:137`，调用点 L1870，confidence=0.70，exact-match，待源码复核）；`ToolHooks::PreToolUse`（`internal/agent/agent.go:146`，调用点 L1889，confidence=0.70，exact-match，待源码复核）；`ToolHooks::PostToolUse`（`internal/agent/agent.go:147`，调用点 L1958，confidence=0.70，exact-match，待源码复核）；`ToolHooks::SubagentStop`（`internal/agent/agent.go:158`，调用点 L1977，confidence=0.70，exact-match，待源码复核）；`Agent::setTodoState`（`internal/agent/agent.go:992`，调用点 L1951，confidence=0.70，instance-method，待源码复核）；`Agent::advanceCanonicalTodo`（`internal/agent/agent.go:1035`，调用点 L1945，confidence=0.70，instance-method，待源码复核）；`Agent::planModeBlocked`（`internal/agent/agent.go:1983`，调用点 L1861，confidence=0.70，instance-method，待源码复核）；`Agent::repeatedSuccessBlock`（`internal/agent/agent.go:1999`，调用点 L1837，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L847，confidence=0.70，instance-method，待源码复核）；`Agent::executeBatch`（`internal/agent/agent.go:1500`，调用点 L1501，confidence=0.70，instance-method，待源码复核）；`Agent::executeOne`（`internal/agent/agent.go:1822`，调用点 L1823，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 8 个文件；`internal/agent`、`internal/evidence`、`internal/instruction`、`internal/jobs`、`internal/memory`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/agent.go`、`internal/evidence/evidence.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.executeOneWithRegistry` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1827、L1869、L1906、L1932、L1976 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、网络/协议、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Agent.executeOneWithRegistry` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-024、FN-005 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/agent.go|*Agent.executeOneWithRegistry` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/agent.go:1826-1981` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.executeOneWithRegistry` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G16 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-024`、`FN-005` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/agent.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/agent.go|*Agent.executeOneWithRegistry` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-005 · 函数有效行数 · `*Agent.executeOneWithRegistry`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-005 |
| Baseline key | `func_lines\|internal/agent/agent.go\|*Agent.executeOneWithRegistry` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 56） |
| 文件 | `internal/agent/agent.go` |
| 代码行 | 1826 |
| 函数/方法/包 | `*Agent.executeOneWithRegistry` |
| Baseline / 当前复测 / 阈值 | 135 / 135 / 80 |
| 超出 | +55，1.69x |
| 同一治理对象关联问题 | `CX-024` |

#### 明确的问题描述

`*Agent.executeOneWithRegistry` 的有效代码行 **135**，超过限制 **80**（+55）。CodeGraph 定义物理范围为 1826-1981 行，并记录 25 个直接下游候选；候选职责分布为 状态与控制（truncateToolOutput、ToolHooks::SubagentStop、Agent::setTodoState）、网络/协议（WithResponseLanguagePreference）、任务/并发（isBackgroundTaskCall、WithQueue、Agent::planModeBlocked、Agent::repeatedSuccessBlock）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::executeOneWithRegistry（internal/agent/agent.go:1826-1981）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `Agent::executeBatchWithRegistry`（`internal/agent/agent.go:1504`，调用点 L1532，confidence=0.70，instance-method，待源码复核）；`Agent::executeOne`（`internal/agent/agent.go:1822`，调用点 L1823，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（14）：** `truncateToolOutput`（`internal/agent/agent.go:2157`，调用点 L1969/L1979，confidence=0.90，exact-match，2 条边）；`Ledger::Record`（`internal/evidence/evidence.go:70`，调用点 L1943/L1949，confidence=0.90，exact-match，2 条边）；`ReceiptFromToolCall`（`internal/evidence/evidence.go:610`，调用点 L1942/L1948，confidence=0.90，import，2 条边）；`withCallContext`（`internal/agent/agent.go:74`，调用点 L1911，confidence=0.90，exact-match）；`isBackgroundTaskCall`（`internal/agent/agent.go:2129`，调用点 L1976，confidence=0.90，exact-match）；`WithResponseLanguagePreference`（`internal/agent/reasoning_language.go:140`，调用点 L1924，confidence=0.90，exact-match）；`WithReasoningLanguagePreference`（`internal/agent/reasoning_language.go:162`，调用点 L1929，confidence=0.90，exact-match）；`WithLedger`（`internal/evidence/evidence.go:537`，调用点 L1913，confidence=0.90，import）
- **待源码复核被调候选（11）：** `Gate::Check`（`internal/agent/agent.go:137`，调用点 L1870，confidence=0.70，exact-match，待源码复核）；`ToolHooks::PreToolUse`（`internal/agent/agent.go:146`，调用点 L1889，confidence=0.70，exact-match，待源码复核）；`ToolHooks::PostToolUse`（`internal/agent/agent.go:147`，调用点 L1958，confidence=0.70，exact-match，待源码复核）；`ToolHooks::SubagentStop`（`internal/agent/agent.go:158`，调用点 L1977，confidence=0.70，exact-match，待源码复核）；`Agent::setTodoState`（`internal/agent/agent.go:992`，调用点 L1951，confidence=0.70，instance-method，待源码复核）；`Agent::advanceCanonicalTodo`（`internal/agent/agent.go:1035`，调用点 L1945，confidence=0.70，instance-method，待源码复核）；`Agent::planModeBlocked`（`internal/agent/agent.go:1983`，调用点 L1861，confidence=0.70，instance-method，待源码复核）；`Agent::repeatedSuccessBlock`（`internal/agent/agent.go:1999`，调用点 L1837，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L847，confidence=0.70，instance-method，待源码复核）；`Agent::executeBatch`（`internal/agent/agent.go:1500`，调用点 L1501，confidence=0.70，instance-method，待源码复核）；`Agent::executeOne`（`internal/agent/agent.go:1822`，调用点 L1823，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 8 个文件；`internal/agent`、`internal/evidence`、`internal/instruction`、`internal/jobs`、`internal/memory`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/agent.go`、`internal/evidence/evidence.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.executeOneWithRegistry` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*Agent.executeOneWithRegistry` 的 135 个有效行按 状态与控制、网络/协议、任务/并发 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L1845 `Translate the tool's optional plan-mode self-report into the policy's`、L1900 `Checkpoint the file this writer is about to change, so the turn can be`、L1962 `Malformed-args failures are a transient model JSON glitch (e.g. options`、L1975 `it doesn't fire here.) SubagentStop lets a hook react to delegated work.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-024、FN-005 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/agent/agent.go|*Agent.executeOneWithRegistry` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/agent.go:1826-1981` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.executeOneWithRegistry` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G16 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-024`、`FN-005` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/agent.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/agent/agent.go|*Agent.executeOneWithRegistry` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-019 · 圈复杂度 · `*Agent.Run`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-019 |
| Baseline key | `complexity\|internal/agent/agent.go\|*Agent.Run` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 54） |
| 文件 | `internal/agent/agent.go` |
| 代码行 | 675 |
| 函数/方法/包 | `*Agent.Run` |
| Baseline / 当前复测 / 阈值 | 38 / 38 / 10 |
| 超出 | +28，3.80x |
| 同一治理对象关联问题 | `FN-002` |

#### 明确的问题描述

`*Agent.Run` 的圈复杂度当前为 **38**，超过限制 **10**（+28，3.80x）。定义范围为 675-879 行；可见控制流热点：L680 `if a.evidence != nil {`；L701 `if strings.TrimSpace(compiledInput) != "" {`；L743 `if err != nil {`；L793 `if len(calls) == 0 {`；L826 `if readiness.applies {`；L868 `if turnMaxSteps > 0 && step+1 >= turnMaxSteps {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（Agent::memoryCompilerRuntime、Agent::executeBatchWithRegistry、MockProvider::Reset、Runtime::StartTurn）、持久化/I-O（RecordReadinessAudit、finalReadinessRetryMessage、finalReadinessCheck::audit、ArchiveSink::Err）、渲染/输出（emptyFinalNotice、Agent::emitMemoryCompilerStats、Agent::withPreviewFileDiffsFrom）、网络/协议（streamRecoveryMessage、Agent::stream、MockProvider::Reset）、任务/并发（Agent::clearSteerQueue、Agent::steerQueueLen）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::Run（internal/agent/agent.go:675-879）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（14）：** `hasVisibleFinalAnswer`（`internal/agent/agent.go:1331`，调用点 L746/L757/L809，confidence=0.90，exact-match，3 条边）；`RecordReadinessAudit`（`internal/event/event.go:347`，调用点 L800/L803/L827，confidence=0.90，import，3 条边）；`userImages`（`internal/agent/agent.go:124`，调用点 L706，confidence=0.90，exact-match）；`midTurnSteerMessage`（`internal/agent/agent.go:477`，调用点 L732，confidence=0.90，exact-match）；`finalReadinessRetryMessage`（`internal/agent/agent.go:1186`，调用点 L805，confidence=0.90，exact-match）；`shouldNudgeExecutorHandoff`（`internal/agent/agent.go:1190`，调用点 L819，confidence=0.90，exact-match）；`executorHandoffRetryMessage`（`internal/agent/agent.go:1322`，调用点 L822，confidence=0.90，exact-match）；`emptyFinalRetryMessage`（`internal/agent/agent.go:1335`，调用点 L815，confidence=0.90，exact-match）
- **待源码复核被调候选（20）：** `Agent::withTurnPreferences`（`internal/agent/agent.go:396`，调用点 L690/L702/L732/L757，confidence=0.70，instance-method，8 条边，待源码复核）；`Agent::maybeCompact`（`internal/agent/compact.go:84`，调用点 L806/L816/L823/L835，confidence=0.70，instance-method，5 条边，待源码复核）；`finalReadinessCheck::audit`（`internal/agent/agent.go:922`，调用点 L800/L803/L827，confidence=0.70，instance-method，3 条边，待源码复核）；`Agent::memoryCitations`（`internal/agent/agent.go:1468`，调用点 L752/L790，confidence=0.70，instance-method，2 条边，待源码复核）；`ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L858/L859，confidence=0.70，instance-method，2 条边，待源码复核）；`Agent::memoryCompilerRuntime`（`internal/agent/agent.go:377`，调用点 L691，confidence=0.70，instance-method，待源码复核）；`Agent::consumeSteer`（`internal/agent/agent.go:512`，调用点 L731，confidence=0.70，instance-method，待源码复核）；`Agent::clearSteerQueue`（`internal/agent/agent.go:524`，调用点 L676，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 11 个文件；`internal/agent`、`internal/event`、`internal/automation`、`internal/memorycompiler`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/agent.go`、`internal/event/event.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.Run` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L680、L713、L769、L811、L868 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Agent.Run` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-019、FN-002 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/agent.go|*Agent.Run` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/agent.go:675-879` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.Run` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G16 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-019`、`FN-002` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/agent.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/agent.go|*Agent.Run` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-002 · 函数有效行数 · `*Agent.Run`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-002 |
| Baseline key | `func_lines\|internal/agent/agent.go\|*Agent.Run` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 54） |
| 文件 | `internal/agent/agent.go` |
| 代码行 | 675 |
| 函数/方法/包 | `*Agent.Run` |
| Baseline / 当前复测 / 阈值 | 172 / 172 / 80 |
| 超出 | +92，2.15x |
| 同一治理对象关联问题 | `CX-019` |

#### 明确的问题描述

`*Agent.Run` 的有效代码行 **172**，超过限制 **80**（+92）。CodeGraph 定义物理范围为 675-879 行，并记录 34 个直接下游候选；候选职责分布为 状态与控制（Agent::memoryCompilerRuntime、Agent::executeBatchWithRegistry、MockProvider::Reset、Runtime::StartTurn）、持久化/I-O（RecordReadinessAudit、finalReadinessRetryMessage、finalReadinessCheck::audit、ArchiveSink::Err）、渲染/输出（emptyFinalNotice、Agent::emitMemoryCompilerStats、Agent::withPreviewFileDiffsFrom）、网络/协议（streamRecoveryMessage、Agent::stream、MockProvider::Reset）、任务/并发（Agent::clearSteerQueue、Agent::steerQueueLen）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::Run（internal/agent/agent.go:675-879）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（14）：** `hasVisibleFinalAnswer`（`internal/agent/agent.go:1331`，调用点 L746/L757/L809，confidence=0.90，exact-match，3 条边）；`RecordReadinessAudit`（`internal/event/event.go:347`，调用点 L800/L803/L827，confidence=0.90，import，3 条边）；`userImages`（`internal/agent/agent.go:124`，调用点 L706，confidence=0.90，exact-match）；`midTurnSteerMessage`（`internal/agent/agent.go:477`，调用点 L732，confidence=0.90，exact-match）；`finalReadinessRetryMessage`（`internal/agent/agent.go:1186`，调用点 L805，confidence=0.90，exact-match）；`shouldNudgeExecutorHandoff`（`internal/agent/agent.go:1190`，调用点 L819，confidence=0.90，exact-match）；`executorHandoffRetryMessage`（`internal/agent/agent.go:1322`，调用点 L822，confidence=0.90，exact-match）；`emptyFinalRetryMessage`（`internal/agent/agent.go:1335`，调用点 L815，confidence=0.90，exact-match）
- **待源码复核被调候选（20）：** `Agent::withTurnPreferences`（`internal/agent/agent.go:396`，调用点 L690/L702/L732/L757，confidence=0.70，instance-method，8 条边，待源码复核）；`Agent::maybeCompact`（`internal/agent/compact.go:84`，调用点 L806/L816/L823/L835，confidence=0.70，instance-method，5 条边，待源码复核）；`finalReadinessCheck::audit`（`internal/agent/agent.go:922`，调用点 L800/L803/L827，confidence=0.70，instance-method，3 条边，待源码复核）；`Agent::memoryCitations`（`internal/agent/agent.go:1468`，调用点 L752/L790，confidence=0.70，instance-method，2 条边，待源码复核）；`ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L858/L859，confidence=0.70，instance-method，2 条边，待源码复核）；`Agent::memoryCompilerRuntime`（`internal/agent/agent.go:377`，调用点 L691，confidence=0.70，instance-method，待源码复核）；`Agent::consumeSteer`（`internal/agent/agent.go:512`，调用点 L731，confidence=0.70，instance-method，待源码复核）；`Agent::clearSteerQueue`（`internal/agent/agent.go:524`，调用点 L676，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 11 个文件；`internal/agent`、`internal/event`、`internal/automation`、`internal/memorycompiler`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/agent.go`、`internal/event/event.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.Run` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*Agent.Run` 的 172 个有效行按 状态与控制、持久化/I-O、渲染/输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L727 `Consume a queued steer and persist it to the session so it`、L782 `input for no cache or coherence gain.`、L857 `the batch results so the session keeps paired tool-call history.`、L877 `up where it left off.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-019、FN-002 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/agent/agent.go|*Agent.Run` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/agent.go:675-879` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.Run` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G16 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-019`、`FN-002` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/agent.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/agent/agent.go|*Agent.Run` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-023 · 圈复杂度 · `*Agent.executeBatchWithRegistry`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-023 |
| Baseline key | `complexity\|internal/agent/agent.go\|*Agent.executeBatchWithRegistry` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 52） |
| 文件 | `internal/agent/agent.go` |
| 代码行 | 1504 |
| 函数/方法/包 | `*Agent.executeBatchWithRegistry` |
| Baseline / 当前复测 / 阈值 | 30 / 30 / 10 |
| 超出 | +20，3.00x |
| 同一治理对象关联问题 | `FN-004` |

#### 明确的问题描述

`*Agent.executeBatchWithRegistry` 的圈复杂度当前为 **30**，超过限制 **10**（+20，3.00x）。定义范围为 1504-1627 行；可见控制流热点：L1505 `if registry == nil {`；L1517 `if ok {`；L1550 `for _, batch := range partitionToolCalls(registry, calls) {`；L1566 `for i := batch.start; i < batch.end; i++ {`；L1588 `for i, c := range calls {`；L1623 `if !cancelled {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（runParallel、Agent::executeOneWithRegistry）、持久化/I-O（ArchiveSink::Err）、渲染/输出（PreviewChange）、任务/并发（runParallel）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::executeBatchWithRegistry（internal/agent/agent.go:1504-1627）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L847，confidence=0.70，instance-method，待源码复核）；`Agent::executeBatch`（`internal/agent/agent.go:1500`，调用点 L1501，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（5）：** `partitionToolCalls`（`internal/agent/agent.go:1671`，调用点 L1550，confidence=0.90，exact-match）；`runParallel`（`internal/agent/agent.go:1697`，调用点 L1556，confidence=0.90，exact-match）；`Turn::RecordToolResults`（`internal/memorycompiler/runtime.go:1721`，调用点 L1621，confidence=0.90，exact-match）；`PreviewChange`（`internal/tool/tool.go:48`，调用点 L1513，confidence=0.90，import）；`NewRegistry`（`internal/tool/tool.go:136`，调用点 L1506，confidence=0.90，import）
- **待源码复核被调候选（4）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L1539/L1551/L1560/L1570，confidence=0.70，instance-method，5 条边，待源码复核）；`Registry::Get`（`internal/tool/tool.go:233`，调用点 L1509/L1590/L1609，confidence=0.80，instance-method，3 条边，待源码复核）；`Agent::applyStormBreaker`（`internal/agent/agent.go:1754`，调用点 L1624，confidence=0.70，instance-method，待源码复核）；`Agent::executeOneWithRegistry`（`internal/agent/agent.go:1826`，调用点 L1532，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L847，confidence=0.70，instance-method，待源码复核）；`Agent::executeBatch`（`internal/agent/agent.go:1500`，调用点 L1501，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/agent`、`internal/memorycompiler`、`internal/tool`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/agent.go`、`internal/memorycompiler/runtime.go`、`internal/tool/tool.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.executeBatchWithRegistry` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1505、L1518、L1555、L1583、L1623 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Agent.executeBatchWithRegistry` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-023、FN-004 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/agent.go|*Agent.executeBatchWithRegistry` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/agent.go:1504-1627` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.executeBatchWithRegistry` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G16 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-023`、`FN-004` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/agent.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/agent.go|*Agent.executeBatchWithRegistry` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-004 · 函数有效行数 · `*Agent.executeBatchWithRegistry`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-004 |
| Baseline key | `func_lines\|internal/agent/agent.go\|*Agent.executeBatchWithRegistry` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 52） |
| 文件 | `internal/agent/agent.go` |
| 代码行 | 1504 |
| 函数/方法/包 | `*Agent.executeBatchWithRegistry` |
| Baseline / 当前复测 / 阈值 | 110 / 110 / 80 |
| 超出 | +30，1.38x |
| 同一治理对象关联问题 | `CX-023` |

#### 明确的问题描述

`*Agent.executeBatchWithRegistry` 的有效代码行 **110**，超过限制 **80**（+30）。CodeGraph 定义物理范围为 1504-1627 行，并记录 9 个直接下游候选；候选职责分布为 状态与控制（runParallel、Agent::executeOneWithRegistry）、持久化/I-O（ArchiveSink::Err）、渲染/输出（PreviewChange）、任务/并发（runParallel）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Agent::executeBatchWithRegistry（internal/agent/agent.go:1504-1627）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L847，confidence=0.70，instance-method，待源码复核）；`Agent::executeBatch`（`internal/agent/agent.go:1500`，调用点 L1501，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（5）：** `partitionToolCalls`（`internal/agent/agent.go:1671`，调用点 L1550，confidence=0.90，exact-match）；`runParallel`（`internal/agent/agent.go:1697`，调用点 L1556，confidence=0.90，exact-match）；`Turn::RecordToolResults`（`internal/memorycompiler/runtime.go:1721`，调用点 L1621，confidence=0.90，exact-match）；`PreviewChange`（`internal/tool/tool.go:48`，调用点 L1513，confidence=0.90，import）；`NewRegistry`（`internal/tool/tool.go:136`，调用点 L1506，confidence=0.90，import）
- **待源码复核被调候选（4）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L1539/L1551/L1560/L1570，confidence=0.70，instance-method，5 条边，待源码复核）；`Registry::Get`（`internal/tool/tool.go:233`，调用点 L1509/L1590/L1609，confidence=0.80，instance-method，3 条边，待源码复核）；`Agent::applyStormBreaker`（`internal/agent/agent.go:1754`，调用点 L1624，confidence=0.70，instance-method，待源码复核）；`Agent::executeOneWithRegistry`（`internal/agent/agent.go:1826`，调用点 L1532，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Agent::Run`（`internal/agent/agent.go:675`，调用点 L847，confidence=0.70，instance-method，待源码复核）；`Agent::executeBatch`（`internal/agent/agent.go:1500`，调用点 L1501，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/agent`、`internal/memorycompiler`、`internal/tool`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/agent.go`、`internal/memorycompiler/runtime.go`、`internal/tool/tool.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*Agent.executeBatchWithRegistry` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*Agent.executeBatchWithRegistry` 的 110 个有效行按 状态与控制、持久化/I-O、渲染/输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L1557 `After parallel execution completes, check if context was cancelled.`、L1567 `Before executing the next tool, check if context was cancelled.`、L1569 `triggered cancellation.`、L1577 `the agent loop can detect the cancellation and exit.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-023、FN-004 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/agent/agent.go|*Agent.executeBatchWithRegistry` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/agent.go:1504-1627` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Agent.executeBatchWithRegistry` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G16 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-023`、`FN-004` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/agent.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/agent/agent.go|*Agent.executeBatchWithRegistry` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-055 · 圈复杂度 · `subSinkForContext`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-055 |
| Baseline key | `complexity\|internal/agent/task.go\|subSinkForContext` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 47） |
| 文件 | `internal/agent/task.go` |
| 代码行 | 869 |
| 函数/方法/包 | `subSinkForContext` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`subSinkForContext` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 869-909 行；可见控制流热点：L870 `if parent == nil {`；L875 `case event.Text:`；L889 `case event.Reasoning, event.Message:`；L892 `case event.ToolDispatch, event.ToolResult, event.ToolProgress:`；L897 `if taskCtx.TreeID != "" && taskCtx.NodeID != "" {`；L903 `if e.UsageSource == "" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function subSinkForContext（internal/agent/task.go:869-909）
- **可信生产调用方（5）：** `ParallelTasksTool::Execute`（`internal/agent/parallel_tasks.go:111`，调用点 L212，confidence=0.90，exact-match）；`TaskTool::Execute`（`internal/agent/task.go:434`，调用点 L494，confidence=0.90，exact-match）；`NestedSink`（`internal/agent/task.go:828`，调用点 L834，confidence=0.90，exact-match）；`subSink`（`internal/agent/task.go:853`，调用点 L859，confidence=0.90，exact-match）；`subSinkFor`（`internal/agent/task.go:865`，调用点 L866，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `FuncSink`（`internal/event/event.go:366`，调用点 L873，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `ParallelTasksTool::Execute`（`internal/agent/parallel_tasks.go:111`，调用点 L212，confidence=0.90，exact-match，待源码复核）；`ReadOnlyTaskTool::Execute`（`internal/agent/task.go:383`，调用点 L419，confidence=0.90，exact-match，待源码复核）；`TaskTool::Execute`（`internal/agent/task.go:434`，调用点 L494，confidence=0.90，exact-match，待源码复核）；`subSinkFor`（`internal/agent/task.go:865`，调用点 L866，confidence=0.90，exact-match，待源码复核）；`Build`（`internal/boot/boot.go:152`，调用点 L749，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/agent`、`internal/event`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/task_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/task.go`、`internal/agent/parallel_tasks.go`、`internal/event/event.go`、`internal/agent/task_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`

#### 待修改项

- [ ] 先冻结 `subSinkForContext` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L870、L875、L892、L897、L903 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `subSinkForContext` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/task.go|subSinkForContext` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/task.go:869-909` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `subSinkForContext` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G16 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/task_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/task.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/task.go|subSinkForContext` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-054 · 圈复杂度 · `ReadOnlySubagentToolRegistry`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-054 |
| Baseline key | `complexity\|internal/agent/task.go\|ReadOnlySubagentToolRegistry` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 45） |
| 文件 | `internal/agent/task.go` |
| 代码行 | 672 |
| 函数/方法/包 | `ReadOnlySubagentToolRegistry` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ReadOnlySubagentToolRegistry` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 672-711 行；可见控制流热点：L677 `for _, e := range exclude {`；L685 `if len(src) == 0 {`；L688 `for _, name := range src {`；L693 `if !ok {`；L696 `if name == "bash" {`；L703 `if u, ok := tl.(tool.PlanModeUntrustedReadOnly); ok && u.PlanModeUntrustedReadOnly() {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ReadOnlySubagentToolRegistry（internal/agent/task.go:672-711）
- **可信生产调用方（2）：** `ReadOnlyTaskTool::Execute`（`internal/agent/task.go:383`，调用点 L410，confidence=0.90，exact-match）；`Build`（`internal/boot/boot.go:152`，调用点 L724，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `SubagentMetaTools`（`internal/agent/task.go:67`，调用点 L673，confidence=0.90，exact-match）；`NewRegistry`（`internal/tool/tool.go:136`，调用点 L680，confidence=0.90，import）
- **待源码复核被调候选（1）：** `Registry::Names`（`internal/tool/tool.go:250`，调用点 L686，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `ReadOnlyTaskTool::Execute`（`internal/agent/task.go:383`，调用点 L410，confidence=0.90，exact-match，待源码复核）；`buildController`（`internal/automation/runner.go:65`，调用点 L66，confidence=0.90，import，待源码复核）；`Build`（`internal/boot/boot.go:152`，调用点 L724，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/agent`、`internal/boot`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/subagent_registry_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/task.go`、`internal/boot/boot.go`、`internal/tool/tool.go`、`internal/agent/subagent_registry_test.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`

#### 待修改项

- [ ] 先冻结 `ReadOnlySubagentToolRegistry` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L677、L685、L689、L696、L703 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `ReadOnlySubagentToolRegistry` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/task.go|ReadOnlySubagentToolRegistry` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/task.go:672-711` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ReadOnlySubagentToolRegistry` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G16 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/subagent_registry_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/task.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/task.go|ReadOnlySubagentToolRegistry` 下降或消失且无新增/恶化。
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
