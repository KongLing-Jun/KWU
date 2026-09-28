# G27 · internal/jobs · jobs 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G27 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/jobs`、`cmd/e2ebench` |
| 涉及文件 | `internal/jobs/jobs.go`<br>`cmd/e2ebench/diff.go` |
| 主要治理目标 | 围绕 `internal/jobs/jobs.go` 等治理对象降低复杂度/体量，统一处理 2 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | jobs / cmd/e2ebench 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 中：认领前确认同包开发 |
| 冲突依据 | 单文件最多集中 7 条问题 |
| 建议处理顺序 | 总序第 27 组；先于 G41 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **45/100**，对象 `internal/jobs/jobs.go` 的分项为严重度 8.0、集中度 6.8、影响面 14.4、可执行性 16.0；最高超限 1.59x，文件内 7 条问题，CodeGraph 记录 6 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 2 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G41，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `FL-014` | `file_lines\|internal/jobs/jobs.go\|` | 文件有效行数 | `internal/jobs/jobs.go` | `internal/jobs/jobs.go` | 1 | 1275/800 | +475（1.59x） | P2 | 6 可信 + 0 待核 caller / 8 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-287` | `complexity\|internal/jobs/jobs.go\|*Manager.StartForSession` | 圈复杂度 | `internal/jobs/jobs.go` | `*Manager.StartForSession` | 242 | 18/10 | +8（1.80x） | P2 | 0 可信 + 3 待核 caller / 6 可信 + 10 待核 callee / 0 unresolved | 待修改 |
| `FN-041` | `func_lines\|internal/jobs/jobs.go\|*Manager.StartForSession` | 函数有效行数 | `internal/jobs/jobs.go` | `*Manager.StartForSession` | 242 | 99/80 | +19（1.24x） | P2 | 0 可信 + 3 待核 caller / 6 可信 + 10 待核 callee / 0 unresolved | 待修改 |
| `CX-002` | `complexity\|cmd/e2ebench/diff.go\|changedLineSet` | 圈复杂度 | `cmd/e2ebench/diff.go` | `changedLineSet` | 418 | 11/10 | +1（1.10x） | P2 | 2 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `ND-002` | `nesting_depth\|cmd/e2ebench/diff.go\|changedLineSet` | 嵌套深度 | `cmd/e2ebench/diff.go` | `changedLineSet` | 418 | 5/4 | +1（1.25x） | P2 | 2 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-001` | `complexity\|cmd/e2ebench/diff.go\|renderDiff` | 圈复杂度 | `cmd/e2ebench/diff.go` | `renderDiff` | 198 | 16/10 | +6（1.60x） | P3 | 1 可信 + 0 待核 caller / 10 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-288` | `complexity\|internal/jobs/jobs.go\|*Manager.SetActiveSessionPath` | 圈复杂度 | `internal/jobs/jobs.go` | `*Manager.SetActiveSessionPath` | 875 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 6 待核 callee / 0 unresolved | 待修改 |
| `CX-291` | `complexity\|internal/jobs/jobs.go\|waitTeardownTargets` | 圈复杂度 | `internal/jobs/jobs.go` | `waitTeardownTargets` | 1341 | 12/10 | +2（1.20x） | P3 | 2 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-290` | `complexity\|internal/jobs/jobs.go\|*Manager.loadSessionArtifacts` | 圈复杂度 | `internal/jobs/jobs.go` | `*Manager.loadSessionArtifacts` | 1142 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-289` | `complexity\|internal/jobs/jobs.go\|*Manager.adoptUnscopedJobsLocked` | 圈复杂度 | `internal/jobs/jobs.go` | `*Manager.adoptUnscopedJobsLocked` | 929 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### FL-014 · 文件有效行数 · `internal/jobs/jobs.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-014 |
| Baseline key | `file_lines\|internal/jobs/jobs.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 45） |
| 文件 | `internal/jobs/jobs.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/jobs/jobs.go` |
| Baseline / 当前复测 / 阈值 | 1275 / 1275 / 800 |
| 超出 | +475，1.59x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/jobs/jobs.go` 当前有效代码行 **1275**，超过限制 **800**（+475）。CodeGraph 在文件中索引到的主要大符号包括 Manager::StartForSession@242-352、Manager::loadSessionArtifacts@1142-1201、Manager::SetActiveSessionPath@875-918、Job::readArtifactSinceOffsetLocked@613-650、Manager::BeginDestroySession@1205-1242、Manager::adoptUnscopedJobsLocked@929-960；文件同时承载的调用影响覆盖 10 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 76 个函数/方法节点。
- **可信生产调用方（6）：** `main`（`cmd/e2ebench/main.go:48`，调用点 L50/L51/L53/L54，confidence=0.90，exact-match，7 条边）；`Controller::BeginDestroySession`（`internal/control/controller.go:2731`，调用点 L2749/L2744/L2741/L2751，confidence=0.90，exact-match，5 条边）；`App::reapOrphanCodeGraph`（`desktop/shared_host.go:92`，调用点 L100/L110/L122/L135，confidence=0.90，exact-match，4 条边）；`ParallelTasksTool::runAsBackgroundJobs`（`internal/agent/parallel_tasks.go:409`，调用点 L478/L410/L414，confidence=0.90，instance-method，3 条边）；`TaskTool::Execute`（`internal/agent/task.go:434`，调用点 L505/L486，confidence=0.90，instance-method，3 条边）；`bash::Execute`（`internal/tool/builtin/bash.go:124`，调用点 L156/L149，confidence=0.90，instance-method，3 条边）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `appendTail`（`internal/jobs/artifacts.go:99`，调用点 L307/L222，confidence=0.90，exact-match，2 条边）；`Done`（`internal/jobs/jobs.go:37`，调用点 L279/L463，confidence=0.90，exact-match，2 条边）；`readMeta`（`internal/jobs/artifacts.go:75`，调用点 L1156，confidence=0.90，exact-match）；`maxJobSeq`（`internal/jobs/artifacts.go:87`，调用点 L1161，confidence=0.90，exact-match）；`ArtifactDir`（`internal/jobs/artifacts.go:21`，调用点 L890，confidence=0.90，exact-match）；`writeMeta`（`internal/jobs/artifacts.go:47`，调用点 L400，confidence=0.90，exact-match）；`renamePath`（`internal/jobs/jobs.go:30`，调用点 L1106，confidence=0.90，exact-match）；`IsNil`（`internal/nilutil/nil.go:6`，调用点 L190，confidence=0.90，import）
- **待源码复核被调候选（1）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L287，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 10 个文件；`internal/jobs`、`cmd/e2ebench`、`internal/control`、`desktop`、`internal/agent`、`internal/tool`、`internal/automation`、`internal/nilutil`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/jobs/jobs_test.go`、`internal/jobs/artifacts_test.go`、`internal/jobs/concurrency_stress_test.go`、`internal/agent/expert_team_tools_test.go`、`internal/acp/server_test.go`、`internal/agent/task_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/control/controller_test.go`、`internal/tool/builtin/bgjobs_test.go`
- **同目录测试候选：** `internal/jobs/main_test.go`、`internal/jobs/output_result_test.go`
- **需同步检查：** `internal/jobs/jobs.go`、`cmd/e2ebench/main.go`、`internal/control/controller.go`、`desktop/shared_host.go`、`internal/agent/parallel_tasks.go`、`internal/jobs/artifacts.go`、`internal/automation/archive_sink.go`、`internal/jobs/jobs_test.go`、`internal/jobs/artifacts_test.go`、`internal/jobs/concurrency_stress_test.go`、`internal/agent/expert_team_tools_test.go`

#### 待修改项

- [ ] 围绕主要符号 `Manager::StartForSession`、`Manager::loadSessionArtifacts`、`Manager::SetActiveSessionPath`、`Job::readArtifactSinceOffsetLocked`、`Manager::BeginDestroySession` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-287、CX-288、CX-289、CX-290、CX-291、FN-041。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/jobs/jobs.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/jobs/jobs.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/jobs/jobs.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/jobs/jobs_test.go`、`internal/jobs/artifacts_test.go`、`internal/jobs/concurrency_stress_test.go`、`internal/agent/expert_team_tools_test.go`、`internal/acp/server_test.go`。
- [ ] 运行相关包测试：`go test ./internal/jobs`。
- [ ] 运行相关包 race：`go test -race ./internal/jobs`。
- [ ] 运行相关包 vet：`go vet ./internal/jobs`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/jobs/jobs.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/jobs/jobs.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-287 · 圈复杂度 · `*Manager.StartForSession`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-287 |
| Baseline key | `complexity\|internal/jobs/jobs.go\|*Manager.StartForSession` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 45） |
| 文件 | `internal/jobs/jobs.go` |
| 代码行 | 242 |
| 函数/方法/包 | `*Manager.StartForSession` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | `FN-041` |

#### 明确的问题描述

`*Manager.StartForSession` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 242-352 行；可见控制流热点：L274 `if m.stalledWarning > 0 {`；L289 `case err != nil:`；L300 `if j.artifactFile != nil {`；L313 `if closeErr := j.artifactFile.Close(); closeErr != nil && j.artifactErr == "" {`；L323 `if moveErr := j.moveArtifactToDirLocked(targetDir); moveErr != nil {`；L344 `if j.artifactPath != "" && j.artifactComplete {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（runRecovered、startedText）、持久化/I-O（ArchiveSink::Err、Manager::openArtifactLocked、Manager::writeJobMetaLocked、Manager::Close）、渲染/输出（Manager::emitIfActive）、任务/并发（jobKey、Job::noteArtifactErr、Manager::openArtifactLocked、Manager::writeJobMetaLocked）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Manager::StartForSession（internal/jobs/jobs.go:242-352）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（3）：** `TaskTool::Execute`（`internal/agent/task.go:434`，调用点 L505，confidence=0.70，instance-method，待源码复核）；`Manager::Start`（`internal/jobs/jobs.go:236`，调用点 L237，confidence=0.70，instance-method，待源码复核）；`bash::Execute`（`internal/tool/builtin/bash.go:124`，调用点 L156，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（6）：** `nowMs`（`internal/jobs/jobs.go:1445`，调用点 L248/L297，confidence=0.90，exact-match，2 条边）；`appendTail`（`internal/jobs/artifacts.go:99`，调用点 L307，confidence=0.90，exact-match）；`Done`（`internal/jobs/jobs.go:37`，调用点 L279，confidence=0.90，exact-match）；`runRecovered`（`internal/jobs/jobs.go:354`，调用点 L280，confidence=0.90，exact-match）；`startedText`（`internal/jobs/jobs.go:1447`，调用点 L271，confidence=0.90，exact-match）；`jobKey`（`internal/jobs/jobs.go:1468`，调用点 L266，confidence=0.90，exact-match）
- **待源码复核被调候选（10）：** `Job::noteArtifactErr`（`internal/jobs/jobs.go:427`，调用点 L324/L329，confidence=0.70，instance-method，2 条边，待源码复核）；`ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L287，confidence=0.70，instance-method，待源码复核）；`Manager::openArtifactLocked`（`internal/jobs/jobs.go:363`，调用点 L249，confidence=0.70，instance-method，待源码复核）；`Manager::writeJobMetaLocked`（`internal/jobs/jobs.go:396`，调用点 L327，confidence=0.70，instance-method，待源码复核）；`Manager::artifactTargetDirForJob`（`internal/jobs/jobs.go:414`，调用点 L310，confidence=0.70，instance-method，待源码复核）；`Job::moveArtifactToDirLocked`（`internal/jobs/jobs.go:440`，调用点 L323，confidence=0.70，instance-method，待源码复核）；`Manager::monitorStalled`（`internal/jobs/jobs.go:462`，调用点 L276，confidence=0.70，instance-method，待源码复核）；`Manager::recordCompletion`（`internal/jobs/jobs.go:495`，调用点 L338，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `TaskTool::Execute`（`internal/agent/task.go:434`，调用点 L505，confidence=0.70，instance-method，待源码复核）；`Manager::Start`（`internal/jobs/jobs.go:236`，调用点 L237，confidence=0.70，instance-method，待源码复核）；`bash::Execute`（`internal/tool/builtin/bash.go:124`，调用点 L156，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/jobs`、`internal/agent`、`internal/tool`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/jobs/artifacts_test.go`、`internal/jobs/jobs_test.go`、`desktop/app_test.go`、`desktop/tabs_runtime_status_test.go`、`internal/acp/server_test.go`、`internal/control/controller_test.go`
- **同目录测试候选：** `internal/jobs/concurrency_stress_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/jobs/main_test.go`、`internal/jobs/output_result_test.go`
- **需同步检查：** `internal/jobs/jobs.go`、`internal/agent/task.go`、`internal/tool/builtin/bash.go`、`internal/jobs/artifacts.go`、`internal/jobs/artifacts_test.go`、`internal/jobs/jobs_test.go`、`desktop/app_test.go`、`desktop/tabs_runtime_status_test.go`

#### 待修改项

- [ ] 逐段核对 L274、L291、L312、L322、L344 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Manager.StartForSession` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-287、FN-041 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/jobs/jobs.go|*Manager.StartForSession` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/jobs/jobs.go:242-352` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Manager.StartForSession` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-287`、`FN-041` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/jobs/artifacts_test.go`、`internal/jobs/jobs_test.go`、`desktop/app_test.go`、`desktop/tabs_runtime_status_test.go`、`internal/acp/server_test.go`。
- [ ] 运行相关包测试：`go test ./internal/jobs`。
- [ ] 运行相关包 race：`go test -race ./internal/jobs`。
- [ ] 运行相关包 vet：`go vet ./internal/jobs`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/jobs/jobs.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/jobs/jobs.go|*Manager.StartForSession` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-041 · 函数有效行数 · `*Manager.StartForSession`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-041 |
| Baseline key | `func_lines\|internal/jobs/jobs.go\|*Manager.StartForSession` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 45） |
| 文件 | `internal/jobs/jobs.go` |
| 代码行 | 242 |
| 函数/方法/包 | `*Manager.StartForSession` |
| Baseline / 当前复测 / 阈值 | 99 / 99 / 80 |
| 超出 | +19，1.24x |
| 同一治理对象关联问题 | `CX-287` |

#### 明确的问题描述

`*Manager.StartForSession` 的有效代码行 **99**，超过限制 **80**（+19）。CodeGraph 定义物理范围为 242-352 行，并记录 16 个直接下游候选；候选职责分布为 状态与控制（runRecovered、startedText）、持久化/I-O（ArchiveSink::Err、Manager::openArtifactLocked、Manager::writeJobMetaLocked、Manager::Close）、渲染/输出（Manager::emitIfActive）、任务/并发（jobKey、Job::noteArtifactErr、Manager::openArtifactLocked、Manager::writeJobMetaLocked）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Manager::StartForSession（internal/jobs/jobs.go:242-352）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（3）：** `TaskTool::Execute`（`internal/agent/task.go:434`，调用点 L505，confidence=0.70，instance-method，待源码复核）；`Manager::Start`（`internal/jobs/jobs.go:236`，调用点 L237，confidence=0.70，instance-method，待源码复核）；`bash::Execute`（`internal/tool/builtin/bash.go:124`，调用点 L156，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（6）：** `nowMs`（`internal/jobs/jobs.go:1445`，调用点 L248/L297，confidence=0.90，exact-match，2 条边）；`appendTail`（`internal/jobs/artifacts.go:99`，调用点 L307，confidence=0.90，exact-match）；`Done`（`internal/jobs/jobs.go:37`，调用点 L279，confidence=0.90，exact-match）；`runRecovered`（`internal/jobs/jobs.go:354`，调用点 L280，confidence=0.90，exact-match）；`startedText`（`internal/jobs/jobs.go:1447`，调用点 L271，confidence=0.90，exact-match）；`jobKey`（`internal/jobs/jobs.go:1468`，调用点 L266，confidence=0.90，exact-match）
- **待源码复核被调候选（10）：** `Job::noteArtifactErr`（`internal/jobs/jobs.go:427`，调用点 L324/L329，confidence=0.70，instance-method，2 条边，待源码复核）；`ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L287，confidence=0.70，instance-method，待源码复核）；`Manager::openArtifactLocked`（`internal/jobs/jobs.go:363`，调用点 L249，confidence=0.70，instance-method，待源码复核）；`Manager::writeJobMetaLocked`（`internal/jobs/jobs.go:396`，调用点 L327，confidence=0.70，instance-method，待源码复核）；`Manager::artifactTargetDirForJob`（`internal/jobs/jobs.go:414`，调用点 L310，confidence=0.70，instance-method，待源码复核）；`Job::moveArtifactToDirLocked`（`internal/jobs/jobs.go:440`，调用点 L323，confidence=0.70，instance-method，待源码复核）；`Manager::monitorStalled`（`internal/jobs/jobs.go:462`，调用点 L276，confidence=0.70，instance-method，待源码复核）；`Manager::recordCompletion`（`internal/jobs/jobs.go:495`，调用点 L338，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `TaskTool::Execute`（`internal/agent/task.go:434`，调用点 L505，confidence=0.70，instance-method，待源码复核）；`Manager::Start`（`internal/jobs/jobs.go:236`，调用点 L237，confidence=0.70，instance-method，待源码复核）；`bash::Execute`（`internal/tool/builtin/bash.go:124`，调用点 L156，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/jobs`、`internal/agent`、`internal/tool`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/jobs/artifacts_test.go`、`internal/jobs/jobs_test.go`、`desktop/app_test.go`、`desktop/tabs_runtime_status_test.go`、`internal/acp/server_test.go`、`internal/control/controller_test.go`
- **同目录测试候选：** `internal/jobs/concurrency_stress_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/jobs/main_test.go`、`internal/jobs/output_result_test.go`
- **需同步检查：** `internal/jobs/jobs.go`、`internal/agent/task.go`、`internal/tool/builtin/bash.go`、`internal/jobs/artifacts.go`、`internal/jobs/artifacts_test.go`、`internal/jobs/jobs_test.go`、`desktop/app_test.go`、`desktop/tabs_runtime_status_test.go`

#### 待修改项

- [ ] 把 `*Manager.StartForSession` 的 99 个有效行按 状态与控制、持久化/I-O、渲染/输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L332 `Queue the drain note (and emit the closing Notice) BEFORE publishing the`、L334 `status flipped to terminal before the note was queued, a Wait could observe`、L335 `completion, skip j.done, and DrainCompletedNote would race ahead of the`、L337 `observed terminal status imply the note is already queued.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-287、FN-041 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/jobs/jobs.go|*Manager.StartForSession` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/jobs/jobs.go:242-352` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Manager.StartForSession` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-287`、`FN-041` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/jobs/artifacts_test.go`、`internal/jobs/jobs_test.go`、`desktop/app_test.go`、`desktop/tabs_runtime_status_test.go`、`internal/acp/server_test.go`。
- [ ] 运行相关包测试：`go test ./internal/jobs`。
- [ ] 运行相关包 race：`go test -race ./internal/jobs`。
- [ ] 运行相关包 vet：`go vet ./internal/jobs`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/jobs/jobs.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/jobs/jobs.go|*Manager.StartForSession` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-002 · 圈复杂度 · `changedLineSet`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-002 |
| Baseline key | `complexity\|cmd/e2ebench/diff.go\|changedLineSet` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 44） |
| 文件 | `cmd/e2ebench/diff.go` |
| 代码行 | 418 |
| 函数/方法/包 | `changedLineSet` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | `ND-002` |

#### 明确的问题描述

`changedLineSet` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 418-453 行；可见控制流热点：L424 `for _, ln := range strings.Split(diff, "\n") {`；L428 `case strings.HasPrefix(ln, "+++ b/"):`；L431 `case strings.HasPrefix(ln, "@@"):`；L437 `for i := 0; i < len(num); i++ {`；L438 `if num[i] < '0' || num[i] > '9' {`；L446 `if file != "" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function changedLineSet（cmd/e2ebench/diff.go:418-453）
- **可信生产调用方（2）：** `changedLineCoverage`（`cmd/e2ebench/diff.go:345`，调用点 L354，confidence=0.90，exact-match）；`runMutation`（`cmd/e2ebench/mutation.go:28`，调用点 L29，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `gitOut`（`cmd/e2ebench/diff.go:636`，调用点 L420，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`cmd/e2ebench`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `cmd/e2ebench/diff.go`、`cmd/e2ebench/mutation.go`

#### 待修改项

- [ ] 逐段核对 L424、L428、L434、L438、L446 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `changedLineSet` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-002、ND-002 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|cmd/e2ebench/diff.go|changedLineSet` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`cmd/e2ebench/diff.go:418-453` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `changedLineSet` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-002`、`ND-002` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 补充或确认覆盖当前职责、错误分支和边界条件的聚焦测试；不得仅依赖全量测试。
- [ ] 运行相关包测试：`go test ./cmd/e2ebench`。
- [ ] 运行相关包 vet：`go vet ./cmd/e2ebench`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- cmd/e2ebench/diff.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|cmd/e2ebench/diff.go|changedLineSet` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-002 · 嵌套深度 · `changedLineSet`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-002 |
| Baseline key | `nesting_depth\|cmd/e2ebench/diff.go\|changedLineSet` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 44） |
| 文件 | `cmd/e2ebench/diff.go` |
| 代码行 | 418 |
| 函数/方法/包 | `changedLineSet` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-002` |

#### 明确的问题描述

`changedLineSet` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L424 `for _, ln := range strings.Split(diff, "\n") {`；L428 `case strings.HasPrefix(ln, "+++ b/"):`；L431 `case strings.HasPrefix(ln, "@@"):`；L437 `for i := 0; i < len(num); i++ {`；L438 `if num[i] < '0' || num[i] > '9' {`；L446 `if file != "" {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function changedLineSet（cmd/e2ebench/diff.go:418-453）
- **可信生产调用方（2）：** `changedLineCoverage`（`cmd/e2ebench/diff.go:345`，调用点 L354，confidence=0.90，exact-match）；`runMutation`（`cmd/e2ebench/mutation.go:28`，调用点 L29，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `gitOut`（`cmd/e2ebench/diff.go:636`，调用点 L420，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`cmd/e2ebench`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `cmd/e2ebench/diff.go`、`cmd/e2ebench/mutation.go`

#### 待修改项

- [ ] 从 L424、L428、L434、L438、L446 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-002、ND-002 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|cmd/e2ebench/diff.go|changedLineSet` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`cmd/e2ebench/diff.go:418-453` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `changedLineSet` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-002`、`ND-002` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 补充或确认覆盖当前职责、错误分支和边界条件的聚焦测试；不得仅依赖全量测试。
- [ ] 运行相关包测试：`go test ./cmd/e2ebench`。
- [ ] 运行相关包 vet：`go vet ./cmd/e2ebench`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- cmd/e2ebench/diff.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|cmd/e2ebench/diff.go|changedLineSet` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-001 · 圈复杂度 · `renderDiff`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-001 |
| Baseline key | `complexity\|cmd/e2ebench/diff.go\|renderDiff` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 42） |
| 文件 | `cmd/e2ebench/diff.go` |
| 代码行 | 198 |
| 函数/方法/包 | `renderDiff` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`renderDiff` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 198-264 行；可见控制流热点：L201 `if r.passed {`；L224 `if len(r.failing) > 0 {`；L236 `if r.attempts <= 1 {`；L242 `if r.sourceTouched > 0 {`；L253 `if strings.TrimSpace(r.testDiff) != "" {`；L259 `if r.runErr != nil {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（truncateFor）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function renderDiff（cmd/e2ebench/diff.go:198-264）
- **可信生产调用方（1）：** `runDiff`（`cmd/e2ebench/diff.go:35`，调用点 L63，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（10）：** `passFail`（`cmd/e2ebench/diff.go:664`，调用点 L211/L218，confidence=0.90，exact-match，2 条边）；`comma`（`cmd/e2ebench/main.go:312`，调用点 L221，confidence=0.90，exact-match，2 条边）；`differentialCell`（`cmd/e2ebench/diff.go:266`，调用点 L212，confidence=0.90，exact-match）；`coverageCell`（`cmd/e2ebench/diff.go:273`，调用点 L216，confidence=0.90，exact-match）；`mutationCell`（`cmd/e2ebench/diff.go:280`，调用点 L217，confidence=0.90，exact-match）；`pinCell`（`cmd/e2ebench/diff.go:291`，调用点 L249，confidence=0.90，exact-match）；`countPins`（`cmd/e2ebench/diff.go:455`，调用点 L207，confidence=0.90，exact-match）；`countAssertionPins`（`cmd/e2ebench/diff.go:465`，调用点 L207，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `tail`（`cmd/e2ebench/diff.go:656`，调用点 L240/L257，confidence=0.70，exact-match，2 条边，待源码复核）；`pct`（`cmd/e2ebench/main.go:305`，调用点 L220，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `main`（`cmd/e2ebench/main.go:48`，调用点 L77，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`cmd/e2ebench`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `cmd/e2ebench/diff.go`、`cmd/e2ebench/main.go`

#### 待修改项

- [ ] 逐段核对 L201、L227、L239、L248、L259 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `renderDiff` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|cmd/e2ebench/diff.go|renderDiff` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`cmd/e2ebench/diff.go:198-264` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `renderDiff` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 补充或确认覆盖当前职责、错误分支和边界条件的聚焦测试；不得仅依赖全量测试。
- [ ] 运行相关包测试：`go test ./cmd/e2ebench`。
- [ ] 运行相关包 race：`go test -race ./cmd/e2ebench`。
- [ ] 运行相关包 vet：`go vet ./cmd/e2ebench`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- cmd/e2ebench/diff.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|cmd/e2ebench/diff.go|renderDiff` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-288 · 圈复杂度 · `*Manager.SetActiveSessionPath`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-288 |
| Baseline key | `complexity\|internal/jobs/jobs.go\|*Manager.SetActiveSessionPath` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/jobs/jobs.go` |
| 代码行 | 875 |
| 函数/方法/包 | `*Manager.SetActiveSessionPath` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Manager.SetActiveSessionPath` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 875-918 行；可见控制流热点：L880 `if parentSession == "" || sessionPath == "" {`；L886 `if _, hasDir := m.artifactDirs[parentSession]; !hasDir && m.hasUnscopedJobsLocked() {`；L897 `if adoptDefault {`；L900 `if err := m.migrateArtifactDirForSession(oldSession, oldDir, newDir); err != nil {`；L909 `if adoptDefault {`；L915 `if !loaded {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Manager::loadSessionArtifacts）、持久化/I-O（Manager::migrateArtifactDirForSession）、任务/并发（Manager::artifactDirLocked、Manager::adoptUnscopedJobsLocked、Manager::hasUnscopedJobsLocked）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Manager::SetActiveSessionPath（internal/jobs/jobs.go:875-918）
- **可信生产调用方（1）：** `Controller::setActiveJobSession`（`internal/control/controller.go:2764`，调用点 L2766，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `ArtifactDir`（`internal/jobs/artifacts.go:21`，调用点 L890，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `Manager::artifactDirLocked`（`internal/jobs/jobs.go:380`，调用点 L884/L887，confidence=0.70，instance-method，2 条边，待源码复核）；`Manager::adoptUnscopedJobsLocked`（`internal/jobs/jobs.go:929`，调用点 L903/L910，confidence=0.70，instance-method，2 条边，待源码复核）；`Manager::hasUnscopedJobsLocked`（`internal/jobs/jobs.go:920`，调用点 L886，confidence=0.70，instance-method，待源码复核）；`Manager::recordArtifactMigrationError`（`internal/jobs/jobs.go:962`，调用点 L906，confidence=0.70，instance-method，待源码复核）；`Manager::migrateArtifactDirForSession`（`internal/jobs/jobs.go:988`，调用点 L900，confidence=0.70，instance-method，待源码复核）；`Manager::loadSessionArtifacts`（`internal/jobs/jobs.go:1142`，调用点 L916，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `New`（`internal/control/controller.go:332`，调用点 L390，confidence=0.70，instance-method，待源码复核）；`Controller::NewSession`（`internal/control/controller.go:1946`，调用点 L1960，confidence=0.70，instance-method，待源码复核）；`Controller::ClearSession`（`internal/control/controller.go:1977`，调用点 L2009，confidence=0.70，instance-method，待源码复核）；`Controller::Branch`（`internal/control/controller.go:2250`，调用点 L2299，confidence=0.70，instance-method，待源码复核）；`Controller::SwitchBranch`（`internal/control/controller.go:2320`，调用点 L2354，confidence=0.70，instance-method，待源码复核）；`Controller::Resume`（`internal/control/controller.go:2442`，调用点 L2452，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/jobs`、`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/jobs/artifacts_test.go`
- **同目录测试候选：** `internal/jobs/concurrency_stress_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/jobs/jobs_test.go`、`internal/jobs/main_test.go`、`internal/jobs/output_result_test.go`
- **需同步检查：** `internal/jobs/jobs.go`、`internal/control/controller.go`、`internal/jobs/artifacts.go`、`internal/jobs/artifacts_test.go`、`internal/jobs/concurrency_stress_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/jobs/jobs_test.go`

#### 待修改项

- [ ] 逐段核对 L880、L895、L900、L901、L915 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Manager.SetActiveSessionPath` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/jobs/jobs.go|*Manager.SetActiveSessionPath` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/jobs/jobs.go:875-918` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Manager.SetActiveSessionPath` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/jobs/artifacts_test.go`。
- [ ] 运行相关包测试：`go test ./internal/jobs`。
- [ ] 运行相关包 vet：`go vet ./internal/jobs`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/jobs/jobs.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/jobs/jobs.go|*Manager.SetActiveSessionPath` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-291 · 圈复杂度 · `waitTeardownTargets`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-291 |
| Baseline key | `complexity\|internal/jobs/jobs.go\|waitTeardownTargets` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/jobs/jobs.go` |
| 代码行 | 1341 |
| 函数/方法/包 | `waitTeardownTargets` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`waitTeardownTargets` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 1341-1372 行；可见控制流热点：L1342 `if ctx == nil {`；L1352 `if len(allDone) > 0 && allDone[0] != nil {`；L1354 `case <-allDone[0]:`；L1362 `for _, target := range targets {`；L1364 `case <-target.done:`；L1367 `case <-timeout:`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function waitTeardownTargets（internal/jobs/jobs.go:1341-1372）
- **可信生产调用方（2）：** `Manager::WaitTeardown`（`internal/jobs/jobs.go:1251`，调用点 L1252，confidence=0.90，exact-match）；`Manager::CloseWithGrace`（`internal/jobs/jobs.go:1321`，调用点 L1328，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `teardownTimedOut`（`internal/jobs/jobs.go:1374`，调用点 L1357/L1359/L1366/L1368，confidence=0.90，exact-match，4 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/jobs`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/jobs/artifacts_test.go`、`internal/jobs/concurrency_stress_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/jobs/jobs_test.go`、`internal/jobs/main_test.go`、`internal/jobs/output_result_test.go`
- **需同步检查：** `internal/jobs/jobs.go`、`internal/jobs/artifacts_test.go`、`internal/jobs/concurrency_stress_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/jobs/jobs_test.go`

#### 待修改项

- [ ] 逐段核对 L1342、L1353、L1358、L1363、L1367 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `waitTeardownTargets` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/jobs/jobs.go|waitTeardownTargets` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/jobs/jobs.go:1341-1372` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `waitTeardownTargets` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/jobs/artifacts_test.go`、`internal/jobs/concurrency_stress_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/jobs/jobs_test.go`、`internal/jobs/main_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/jobs`。
- [ ] 运行相关包 race：`go test -race ./internal/jobs`。
- [ ] 运行相关包 vet：`go vet ./internal/jobs`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/jobs/jobs.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/jobs/jobs.go|waitTeardownTargets` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-290 · 圈复杂度 · `*Manager.loadSessionArtifacts`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-290 |
| Baseline key | `complexity\|internal/jobs/jobs.go\|*Manager.loadSessionArtifacts` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/jobs/jobs.go` |
| 代码行 | 1142 |
| 函数/方法/包 | `*Manager.loadSessionArtifacts` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Manager.loadSessionArtifacts` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 1142-1201 行；可见控制流热点：L1144 `if err != nil {`；L1153 `if entry.IsDir() || filepath.Ext(entry.Name()) != jobMetaExt {`；L1157 `if err != nil || strings.TrimSpace(meta.ID) == "" {`；L1167 `if strings.TrimSpace(meta.LogPath) != "" {`；L1189 `for _, j := range loaded {`；L1197 `if maxSeq > m.seq {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readMeta）、任务/并发（maxJobSeq、jobKey）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Manager::loadSessionArtifacts（internal/jobs/jobs.go:1142-1201）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Manager::SetActiveSessionPath`（`internal/jobs/jobs.go:875`，调用点 L916，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `readMeta`（`internal/jobs/artifacts.go:75`，调用点 L1156，confidence=0.90，exact-match）；`maxJobSeq`（`internal/jobs/artifacts.go:87`，调用点 L1161，confidence=0.90，exact-match）；`jobKey`（`internal/jobs/jobs.go:1468`，调用点 L1190，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/jobs`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/jobs/artifacts_test.go`、`internal/jobs/concurrency_stress_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/jobs/jobs_test.go`、`internal/jobs/main_test.go`、`internal/jobs/output_result_test.go`
- **需同步检查：** `internal/jobs/jobs.go`、`internal/jobs/artifacts.go`、`internal/jobs/artifacts_test.go`、`internal/jobs/concurrency_stress_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/jobs/jobs_test.go`

#### 待修改项

- [ ] 逐段核对 L1144、L1153、L1161、L1189、L1197 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Manager.loadSessionArtifacts` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/jobs/jobs.go|*Manager.loadSessionArtifacts` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/jobs/jobs.go:1142-1201` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Manager.loadSessionArtifacts` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41 处理。
- 重点保护：并发、取消或事件顺序；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/jobs/artifacts_test.go`、`internal/jobs/concurrency_stress_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/jobs/jobs_test.go`、`internal/jobs/main_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/jobs`。
- [ ] 运行相关包 race：`go test -race ./internal/jobs`。
- [ ] 运行相关包 vet：`go vet ./internal/jobs`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/jobs/jobs.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/jobs/jobs.go|*Manager.loadSessionArtifacts` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-289 · 圈复杂度 · `*Manager.adoptUnscopedJobsLocked`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-289 |
| Baseline key | `complexity\|internal/jobs/jobs.go\|*Manager.adoptUnscopedJobsLocked` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 31） |
| 文件 | `internal/jobs/jobs.go` |
| 代码行 | 929 |
| 函数/方法/包 | `*Manager.adoptUnscopedJobsLocked` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Manager.adoptUnscopedJobsLocked` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 929-960 行；可见控制流热点：L931 `if parentSession == "" {`；L934 `for i := range m.completed {`；L939 `for oldKey, j := range m.jobs {`；L940 `if j == nil || strings.TrimSpace(j.SessionID) != "" {`；L954 `for i, key := range m.order {`；L955 `if key == oldKey {`。CodeGraph 记录的下游职责候选主要涉及 任务/并发（jobKey）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Manager::adoptUnscopedJobsLocked（internal/jobs/jobs.go:929-960）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Manager::SetActiveSessionPath`（`internal/jobs/jobs.go:875`，调用点 L903/L910，confidence=0.70，instance-method，2 条边，待源码复核）
- **可信被调用函数（1）：** `jobKey`（`internal/jobs/jobs.go:1468`，调用点 L943，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/jobs`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/jobs/artifacts_test.go`、`internal/jobs/concurrency_stress_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/jobs/jobs_test.go`、`internal/jobs/main_test.go`、`internal/jobs/output_result_test.go`
- **需同步检查：** `internal/jobs/jobs.go`、`internal/jobs/artifacts_test.go`、`internal/jobs/concurrency_stress_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/jobs/jobs_test.go`

#### 待修改项

- [ ] 逐段核对 L931、L935、L940、L944、L955 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Manager.adoptUnscopedJobsLocked` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/jobs/jobs.go|*Manager.adoptUnscopedJobsLocked` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/jobs/jobs.go:929-960` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Manager.adoptUnscopedJobsLocked` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/jobs/artifacts_test.go`、`internal/jobs/concurrency_stress_test.go`、`internal/jobs/jobs_extra_test.go`、`internal/jobs/jobs_test.go`、`internal/jobs/main_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/jobs`。
- [ ] 运行相关包 vet：`go vet ./internal/jobs`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/jobs/jobs.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/jobs/jobs.go|*Manager.adoptUnscopedJobsLocked` 下降或消失且无新增/恶化。
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
