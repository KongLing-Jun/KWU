# G28 · internal/automation · schedule 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G28 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/automation` |
| 涉及文件 | `internal/automation/migration.go`<br>`internal/automation/engine.go`<br>`internal/automation/schedule.go`<br>`internal/automation/runner.go`<br>`internal/automation/business.go` |
| 主要治理目标 | 围绕 `internal/automation/schedule.go` 等治理对象降低复杂度/体量，统一处理 1 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | automation 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/expert-202607081956`、`origin/expert-202607082134` |
| 建议处理顺序 | 总序第 28 组；先于 G46 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容；返回值、错误语义和现有调用顺序；权限、安全或凭据边界；并发、取消或事件顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **44/100**，对象 `MigrateHeartbeatIfNeeded` 的分项为严重度 6.1、集中度 8.6、影响面 8.9、可执行性 20.0；最高超限 1.50x，文件内 2 条问题，CodeGraph 记录 0 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数，必须合并治理。

### 治理边界与组间关系

本组涉及 5 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G46，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-069` | `complexity\|internal/automation/migration.go\|MigrateHeartbeatIfNeeded` | 圈复杂度 | `internal/automation/migration.go` | `MigrateHeartbeatIfNeeded` | 55 | 15/10 | +5（1.50x） | P2 | 0 可信 + 0 待核 caller / 8 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `FN-010` | `func_lines\|internal/automation/migration.go\|MigrateHeartbeatIfNeeded` | 函数有效行数 | `internal/automation/migration.go` | `MigrateHeartbeatIfNeeded` | 55 | 118/80 | +38（1.48x） | P2 | 0 可信 + 0 待核 caller / 8 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-063` | `complexity\|internal/automation/engine.go\|*Engine.runTask` | 圈复杂度 | `internal/automation/engine.go` | `*Engine.runTask` | 161 | 18/10 | +8（1.80x） | P3 | 0 可信 + 2 待核 caller / 9 可信 + 10 待核 callee / 4 unresolved | 待修改 |
| `CX-072` | `complexity\|internal/automation/schedule.go\|previousCalendarOccurrence` | 圈复杂度 | `internal/automation/schedule.go` | `previousCalendarOccurrence` | 157 | 15/10 | +5（1.50x） | P3 | 1 可信 + 0 待核 caller / 6 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-073` | `complexity\|internal/automation/schedule.go\|nextCalendarOccurrenceAfter` | 圈复杂度 | `internal/automation/schedule.go` | `nextCalendarOccurrenceAfter` | 203 | 15/10 | +5（1.50x） | P3 | 1 可信 + 0 待核 caller / 6 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-071` | `complexity\|internal/automation/schedule.go\|withinTimeWindow` | 圈复杂度 | `internal/automation/schedule.go` | `withinTimeWindow` | 117 | 17/10 | +7（1.70x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-070` | `complexity\|internal/automation/runner.go\|*BootRunner.Run` | 圈复杂度 | `internal/automation/runner.go` | `*BootRunner.Run` | 69 | 16/10 | +6（1.60x） | P3 | 0 可信 + 0 待核 caller / 16 可信 + 1 待核 callee / 22 unresolved | 待修改 |
| `CX-064` | `complexity\|internal/automation/engine.go\|*Engine.finishRun` | 圈复杂度 | `internal/automation/engine.go` | `*Engine.finishRun` | 254 | 16/10 | +6（1.60x） | P3 | 0 可信 + 2 待核 caller / 3 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `FL-003` | `file_lines\|internal/automation/business.go\|` | 文件有效行数 | `internal/automation/business.go` | `internal/automation/business.go` | 1 | 1055/800 | +255（1.32x） | P3 | 9 可信 + 0 待核 caller / 10 可信 + 6 待核 callee / 201 unresolved | 待修改 |
| `CX-060` | `complexity\|internal/automation/business.go\|ValidateBusinessDiagnostics` | 圈复杂度 | `internal/automation/business.go` | `ValidateBusinessDiagnostics` | 284 | 14/10 | +4（1.40x） | P3 | 2 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 11 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-069 · 圈复杂度 · `MigrateHeartbeatIfNeeded`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-069 |
| Baseline key | `complexity\|internal/automation/migration.go\|MigrateHeartbeatIfNeeded` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 44） |
| 文件 | `internal/automation/migration.go` |
| 代码行 | 55 |
| 函数/方法/包 | `MigrateHeartbeatIfNeeded` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | `FN-010` |

#### 明确的问题描述

`MigrateHeartbeatIfNeeded` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 55-178 行；可见控制流热点：L56 `if err := ctx.Err(); err != nil {`；L66 `if errors.Is(err, os.ErrNotExist) {`；L75 `if marker.SourceHash == sourceHash {`；L110 `if err != nil {`；L140 `if err := ctx.Err(); err != nil {`；L157 `if err := writeHeartbeatMigrationMarker(store, marker); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（decodeLegacyHeartbeatMigration、Store::LoadTasks）、持久化/I-O（writeHeartbeatMigrationMarker、readHeartbeatMigrationMarker、ArchiveSink::Err、Store::LoadTasks）、任务/并发（mergeHeartbeatMigrationTasks、Store::LoadTasks、Store::SaveTasks）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function MigrateHeartbeatIfNeeded（internal/automation/migration.go:55-178）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `heartbeatMigrationFailed`（`internal/automation/migration.go:255`，调用点 L57/L61/L70/L111，confidence=0.90，exact-match，7 条边）；`writeHeartbeatMigrationMarker`（`internal/automation/migration.go:234`，调用点 L125/L157，confidence=0.90，exact-match，2 条边）；`heartbeatMigrationSkippedSummary`（`internal/automation/migration.go:264`，调用点 L114/L155，confidence=0.90，exact-match，2 条边）；`mergeHeartbeatMigrationTasks`（`internal/automation/migration.go:200`，调用点 L139，confidence=0.90，exact-match）；`readHeartbeatMigrationMarker`（`internal/automation/migration.go:222`，调用点 L74，confidence=0.90，exact-match）；`heartbeatMigrationHash`（`internal/automation/migration.go:250`，调用点 L72，confidence=0.90，exact-match）；`decodeLegacyHeartbeatMigration`（`internal/automation/migration_heartbeat.go:87`，调用点 L109，confidence=0.90，exact-match）；`MemoryUserDir`（`internal/config/paths.go:491`，调用点 L64，confidence=0.90，import）
- **待源码复核被调候选（3）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L56/L140，confidence=0.70，instance-method，2 条边，待源码复核）；`Store::LoadTasks`（`internal/automation/store.go:54`，调用点 L135，confidence=0.80，instance-method，待源码复核）；`Store::SaveTasks`（`internal/automation/store.go:74`，调用点 L143，confidence=0.80，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/automation`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/automation/migration_test.go`
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/migration.go`、`internal/automation/migration_test.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`

#### 待修改项

- [ ] 逐段核对 L56、L69、L76、L136、L157 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `MigrateHeartbeatIfNeeded` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-069、FN-010 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/migration.go|MigrateHeartbeatIfNeeded` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/migration.go:55-178` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `MigrateHeartbeatIfNeeded` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G46 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-069`、`FN-010` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/automation/migration_test.go`。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/migration.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/migration.go|MigrateHeartbeatIfNeeded` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-010 · 函数有效行数 · `MigrateHeartbeatIfNeeded`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-010 |
| Baseline key | `func_lines\|internal/automation/migration.go\|MigrateHeartbeatIfNeeded` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 44） |
| 文件 | `internal/automation/migration.go` |
| 代码行 | 55 |
| 函数/方法/包 | `MigrateHeartbeatIfNeeded` |
| Baseline / 当前复测 / 阈值 | 118 / 118 / 80 |
| 超出 | +38，1.48x |
| 同一治理对象关联问题 | `CX-069` |

#### 明确的问题描述

`MigrateHeartbeatIfNeeded` 的有效代码行 **118**，超过限制 **80**（+38）。CodeGraph 定义物理范围为 55-178 行，并记录 11 个直接下游候选；候选职责分布为 配置/解析（decodeLegacyHeartbeatMigration、Store::LoadTasks）、持久化/I-O（writeHeartbeatMigrationMarker、readHeartbeatMigrationMarker、ArchiveSink::Err、Store::LoadTasks）、任务/并发（mergeHeartbeatMigrationTasks、Store::LoadTasks、Store::SaveTasks）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function MigrateHeartbeatIfNeeded（internal/automation/migration.go:55-178）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `heartbeatMigrationFailed`（`internal/automation/migration.go:255`，调用点 L57/L61/L70/L111，confidence=0.90，exact-match，7 条边）；`writeHeartbeatMigrationMarker`（`internal/automation/migration.go:234`，调用点 L125/L157，confidence=0.90，exact-match，2 条边）；`heartbeatMigrationSkippedSummary`（`internal/automation/migration.go:264`，调用点 L114/L155，confidence=0.90，exact-match，2 条边）；`mergeHeartbeatMigrationTasks`（`internal/automation/migration.go:200`，调用点 L139，confidence=0.90，exact-match）；`readHeartbeatMigrationMarker`（`internal/automation/migration.go:222`，调用点 L74，confidence=0.90，exact-match）；`heartbeatMigrationHash`（`internal/automation/migration.go:250`，调用点 L72，confidence=0.90，exact-match）；`decodeLegacyHeartbeatMigration`（`internal/automation/migration_heartbeat.go:87`，调用点 L109，confidence=0.90，exact-match）；`MemoryUserDir`（`internal/config/paths.go:491`，调用点 L64，confidence=0.90，import）
- **待源码复核被调候选（3）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L56/L140，confidence=0.70，instance-method，2 条边，待源码复核）；`Store::LoadTasks`（`internal/automation/store.go:54`，调用点 L135，confidence=0.80，instance-method，待源码复核）；`Store::SaveTasks`（`internal/automation/store.go:74`，调用点 L143，confidence=0.80，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/automation`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/automation/migration_test.go`
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/migration.go`、`internal/automation/migration_test.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`

#### 待修改项

- [ ] 把 `MigrateHeartbeatIfNeeded` 的 118 个有效行按 配置/解析、持久化/I-O、任务/并发 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-069、FN-010 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/automation/migration.go|MigrateHeartbeatIfNeeded` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/migration.go:55-178` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `MigrateHeartbeatIfNeeded` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G46 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-069`、`FN-010` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/automation/migration_test.go`。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/migration.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/automation/migration.go|MigrateHeartbeatIfNeeded` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-063 · 圈复杂度 · `*Engine.runTask`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-063 |
| Baseline key | `complexity\|internal/automation/engine.go\|*Engine.runTask` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 41） |
| 文件 | `internal/automation/engine.go` |
| 代码行 | 161 |
| 函数/方法/包 | `*Engine.runTask` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Engine.runTask` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 161-226 行；可见控制流热点：L164 `if runID == "" {`；L173 `if err != nil || !ok {`；L183 `if err != nil || running {`；L192 `if err != nil {`；L205 `if !run.Condition.ShouldRun {`；L212 `if err := e.store.SaveRunManifest(run); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Engine::reloadTask）、状态与控制（Store::SaveRunManifest、Store::createRun、newRunID、conditionRunResult）、持久化/I-O（Store::SaveRunManifest、Store::createRun、NewArchiveSink、Store::AcquireLease）、渲染/输出（ArchiveSink::EmitRunStatus）、任务/并发（Engine::reloadTask、Engine::taskHasRunningRun）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Engine::runTask（internal/automation/engine.go:161-226）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `Engine::Tick`（`internal/automation/engine.go:61`，调用点 L77，confidence=0.70，instance-method，待源码复核）；`Engine::Trigger`（`internal/automation/engine.go:85`，调用点 L99，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（9）：** `Store::SaveRunManifest`（`internal/automation/archive.go:130`，调用点 L200/L212，confidence=0.90，exact-match，2 条边）；`Store::createRun`（`internal/automation/archive.go:90`，调用点 L187，confidence=0.90，exact-match）；`newRunID`（`internal/automation/archive.go:287`，调用点 L165，confidence=0.90，exact-match）；`NewArchiveSink`（`internal/automation/archive_sink.go:50`，调用点 L191，confidence=0.90，exact-match）；`conditionRunResult`（`internal/automation/engine.go:236`，调用点 L206，confidence=0.90，exact-match）；`Store::AcquireLease`（`internal/automation/lease.go:35`，调用点 L167，confidence=0.90，exact-match）；`Store::ReleaseLease`（`internal/automation/lease.go:103`，调用点 L170，confidence=0.90，exact-match）；`failedRunResult`（`internal/automation/runner.go:241`，调用点 L193，confidence=0.90，exact-match）
- **待源码复核被调候选（10）：** `Selector::now`（`internal/profile/selector.go:291`，调用点 L162/L177/L197/L204，confidence=0.70，instance-method，5 条边，待源码复核）；`ArchiveSink::EmitRunStatus`（`internal/automation/archive_sink.go:77`，调用点 L196/L203/L215，confidence=0.70，instance-method，3 条边，待源码复核）；`Engine::finishRunWithArchive`（`internal/automation/engine.go:312`，调用点 L206/L225，confidence=0.70，instance-method，2 条边，待源码复核）；`RunManifest::Transition`（`internal/automation/model.go:316`，调用点 L197/L209，confidence=0.70，instance-method，2 条边，待源码复核）；`Checker::Check`（`internal/automation/condition.go:26`，调用点 L204，confidence=0.70，exact-match，待源码复核）；`Runner::Run`（`internal/automation/engine.go:19`，调用点 L217，confidence=0.70，exact-match，待源码复核）；`Engine::conditionChecker`（`internal/automation/engine.go:228`，调用点 L204，confidence=0.70，instance-method，待源码复核）；`Engine::finishRun`（`internal/automation/engine.go:254`，调用点 L193，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Engine::Start`（`internal/automation/engine.go:106`，调用点 L127，confidence=0.70，instance-method，待源码复核）；`service::TriggerNow`（`internal/automation/service.go:291`，调用点 L304，confidence=0.70，exact-match，待源码复核）
- **未解析调用：** 4 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 9 个文件；`internal/automation`、`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/engine.go`、`internal/automation/archive.go`、`internal/automation/archive_sink.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 逐段核对 L164、L176、L188、L200、L212 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Engine.runTask` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/engine.go|*Engine.runTask` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/engine.go:161-226` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Engine.runTask` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G46 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/engine.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/engine.go|*Engine.runTask` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-072 · 圈复杂度 · `previousCalendarOccurrence`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-072 |
| Baseline key | `complexity\|internal/automation/schedule.go\|previousCalendarOccurrence` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 41） |
| 文件 | `internal/automation/schedule.go` |
| 代码行 | 157 |
| 函数/方法/包 | `previousCalendarOccurrence` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`previousCalendarOccurrence` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 157-201 行；可见控制流热点：L158 `switch frequency {`；L161 `if candidate.After(now) {`；L171 `for offset := 0; offset < 7; offset++ {`；L177 `if !candidate.After(now) && (best.IsZero() || candidate.After(best)) {`；L188 `case "yearly":`；L194 `if !ok {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseMonthDays、parseMonths、parseScheduleWeekdays）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function previousCalendarOccurrence（internal/automation/schedule.go:157-201）
- **可信生产调用方（1）：** `dueCalendar`（`internal/automation/schedule.go:78`，调用点 L85，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `parseMonthDays`（`internal/automation/schedule.go:307`，调用点 L183/L193，confidence=0.90，exact-match，2 条边）；`dateAtSchedule`（`internal/automation/schedule.go:402`，调用点 L160/L176，confidence=0.90，exact-match，2 条边）；`previousMonthlyOccurrence`（`internal/automation/schedule.go:249`，调用点 L187，confidence=0.90，exact-match）；`previousYearlyOccurrence`（`internal/automation/schedule.go:277`，调用点 L197，confidence=0.90，exact-match）；`parseMonths`（`internal/automation/schedule.go:321`，调用点 L189，confidence=0.90，exact-match）；`parseScheduleWeekdays`（`internal/automation/schedule.go:335`，调用点 L166，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/schedule.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 逐段核对 L158、L165、L173、L184、L194 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `previousCalendarOccurrence` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/schedule.go|previousCalendarOccurrence` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/schedule.go:157-201` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `previousCalendarOccurrence` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G46 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/schedule.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/schedule.go|previousCalendarOccurrence` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-073 · 圈复杂度 · `nextCalendarOccurrenceAfter`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-073 |
| Baseline key | `complexity\|internal/automation/schedule.go\|nextCalendarOccurrenceAfter` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 41） |
| 文件 | `internal/automation/schedule.go` |
| 代码行 | 203 |
| 函数/方法/包 | `nextCalendarOccurrenceAfter` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`nextCalendarOccurrenceAfter` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 203-247 行；可见控制流热点：L204 `switch frequency {`；L207 `if !candidate.After(after) {`；L217 `for offset := 0; offset <= 7; offset++ {`；L223 `if candidate.After(after) && (best.IsZero() || candidate.Before(best)) {`；L234 `case "yearly":`；L240 `if !ok {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseMonthDays、parseMonths、parseScheduleWeekdays）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function nextCalendarOccurrenceAfter（internal/automation/schedule.go:203-247）
- **可信生产调用方（1）：** `dueCalendar`（`internal/automation/schedule.go:78`，调用点 L95/L99，confidence=0.90，exact-match，2 条边）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `parseMonthDays`（`internal/automation/schedule.go:307`，调用点 L229/L239，confidence=0.90，exact-match，2 条边）；`dateAtSchedule`（`internal/automation/schedule.go:402`，调用点 L206/L222，confidence=0.90，exact-match，2 条边）；`nextMonthlyOccurrence`（`internal/automation/schedule.go:263`，调用点 L233，confidence=0.90，exact-match）；`nextYearlyOccurrence`（`internal/automation/schedule.go:292`，调用点 L243，confidence=0.90，exact-match）；`parseMonths`（`internal/automation/schedule.go:321`，调用点 L235，confidence=0.90，exact-match）；`parseScheduleWeekdays`（`internal/automation/schedule.go:335`，调用点 L212，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/schedule.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 逐段核对 L204、L211、L219、L230、L240 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `nextCalendarOccurrenceAfter` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/schedule.go|nextCalendarOccurrenceAfter` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/schedule.go:203-247` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `nextCalendarOccurrenceAfter` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G46 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/schedule.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/schedule.go|nextCalendarOccurrenceAfter` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-071 · 圈复杂度 · `withinTimeWindow`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-071 |
| Baseline key | `complexity\|internal/automation/schedule.go\|withinTimeWindow` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 40） |
| 文件 | `internal/automation/schedule.go` |
| 代码行 | 117 |
| 函数/方法/包 | `withinTimeWindow` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`withinTimeWindow` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 117-155 行；可见控制流热点：L119 `if err != nil {`；L126 `if !startOK && !endOK {`；L131 `if minutes >= start {`；L137 `if minutes < end {`；L143 `if minutes >= start && minutes < end {`；L151 `if minutes >= start || minutes < end {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseOptionalClock）、任务/并发（parseOptionalClock）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function withinTimeWindow（internal/automation/schedule.go:117-155）
- **可信生产调用方（1）：** `Due`（`internal/automation/schedule.go:20`，调用点 L26，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `timeAtMinutes`（`internal/automation/schedule.go:398`，调用点 L134/L140/L147/L149，confidence=0.90，exact-match，5 条边）；`parseOptionalClock`（`internal/automation/schedule.go:368`，调用点 L118/L122，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/schedule.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 逐段核对 L119、L130、L136、L143、L151 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `withinTimeWindow` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/schedule.go|withinTimeWindow` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/schedule.go:117-155` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `withinTimeWindow` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G46 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/schedule.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/schedule.go|withinTimeWindow` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-070 · 圈复杂度 · `*BootRunner.Run`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-070 |
| Baseline key | `complexity\|internal/automation/runner.go\|*BootRunner.Run` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/automation/runner.go` |
| 代码行 | 69 |
| 函数/方法/包 | `*BootRunner.Run` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*BootRunner.Run` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 69-151 行；可见控制流热点：L71 `if runDir == "" {`；L85 `if sink == nil {`；L92 `if sink == nil {`；L111 `if pinner, ok := ctrl.(interface{ EnsureSessionPath() }); ok {`；L120 `if archive != nil {`；L144 `if archive != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeApprovalMode、parseApprovalTimeout）、状态与控制（failedRunResult、runnerFailedRunResult、runResultFromExecution、classifyRunError）、持久化/I-O（NewArchiveSink、archiveMeta、archiveSinkErr、WriteResult）、渲染/输出（EmitAutomationHelperNotice、emitRunError）、网络/协议（resultEventsFromSink）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method BootRunner::Run（internal/automation/runner.go:69-151）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（16）：** `failedRunResult`（`internal/automation/runner.go:241`，调用点 L73/L77/L88，confidence=0.90，exact-match，3 条边）；`runnerFailedRunResult`（`internal/automation/runner.go:278`，调用点 L107/L117，confidence=0.90，exact-match，2 条边）；`NewArchiveSink`（`internal/automation/archive_sink.go:50`，调用点 L86，confidence=0.90，exact-match）；`normalizeApprovalMode`（`internal/automation/model.go:286`，调用点 L114，confidence=0.90，exact-match）；`PromptCompiler::CompileWithOptions`（`internal/automation/prompt.go:22`，调用点 L126，confidence=0.90，exact-match）；`ExtractResult`（`internal/automation/result.go:21`，调用点 L130，confidence=0.90，exact-match）；`PrepareAutomationAssist`（`internal/automation/runner.go:154`，调用点 L115，confidence=0.90，exact-match）；`EmitAutomationHelperNotice`（`internal/automation/runner.go:196`，调用点 L125，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `WriteResult`（`internal/automation/result.go:35`，调用点 L137，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 22 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/runner.go`、`internal/automation/archive_sink.go`、`internal/automation/model.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 逐段核对 L71、L85、L104、L120、L144 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*BootRunner.Run` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/runner.go|*BootRunner.Run` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/runner.go:69-151` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*BootRunner.Run` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G46 处理。
- 重点保护：持久化、迁移或文件格式兼容；权限、安全或凭据边界。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/runner.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/runner.go|*BootRunner.Run` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-064 · 圈复杂度 · `*Engine.finishRun`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-064 |
| Baseline key | `complexity\|internal/automation/engine.go\|*Engine.finishRun` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 38） |
| 文件 | `internal/automation/engine.go` |
| 代码行 | 254 |
| 函数/方法/包 | `*Engine.finishRun` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Engine.finishRun` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 254-306 行；可见控制流热点：L256 `if status == "" {`；L265 `if result.ResultPath != "" {`；L272 `if run.Status == RunStatusSkipped && run.SkipReason == "" {`；L284 `if result.Status != "" {`；L292 `if run.Error == "" {`；L302 `if shouldRecordTaskRun(run) {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（Store::SaveRunManifest、shouldRecordTaskRun、classifyRunError、Engine::recordTaskRun）、持久化/I-O（Store::SaveRunManifest）、任务/并发（shouldRecordTaskRun、Engine::recordTaskRun）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Engine::finishRun（internal/automation/engine.go:254-306）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `Engine::runTask`（`internal/automation/engine.go:161`，调用点 L193，confidence=0.70，instance-method，待源码复核）；`Engine::finishRunWithArchive`（`internal/automation/engine.go:312`，调用点 L339，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `Store::SaveRunManifest`（`internal/automation/archive.go:130`，调用点 L299，confidence=0.90，exact-match）；`shouldRecordTaskRun`（`internal/automation/engine.go:308`，调用点 L302，confidence=0.90，exact-match）；`classifyRunError`（`internal/automation/runner.go:291`，调用点 L283，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `Engine::recordTaskRun`（`internal/automation/engine.go:371`，调用点 L303，confidence=0.70，instance-method，待源码复核）；`RunManifest::Transition`（`internal/automation/model.go:316`，调用点 L259，confidence=0.70，instance-method，待源码复核）；`Selector::now`（`internal/profile/selector.go:291`，调用点 L259，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/automation`、`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/engine.go`、`internal/automation/archive.go`、`internal/automation/runner.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 逐段核对 L256、L265、L282、L292、L302 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Engine.finishRun` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/engine.go|*Engine.finishRun` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/engine.go:254-306` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Engine.finishRun` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G46 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/engine.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/engine.go|*Engine.finishRun` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FL-003 · 文件有效行数 · `internal/automation/business.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-003 |
| Baseline key | `file_lines\|internal/automation/business.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/automation/business.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/automation/business.go` |
| Baseline / 当前复测 / 阈值 | 1055 / 1055 / 800 |
| 超出 | +255，1.32x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/automation/business.go` 当前有效代码行 **1055**，超过限制 **800**（+255）。CodeGraph 在文件中索引到的主要大符号包括 businessMetrics@665-723、WeeklyBusinessDiagnosticsTask@204-256、businessRecordFromRow@561-613、businessDiagnosticsMarkdown@732-780、BusinessDiagnosticsRunner::Run@157-202、readBusinessHTTP@456-495；文件同时承载的调用影响覆盖 14 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 48 个函数/方法节点。
- **可信生产调用方（9）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L867/L1001/L1049/L1085，confidence=0.90，exact-match，5 条边）；`ValidateTask`（`internal/automation/model.go:246`，调用点 L266，confidence=0.90，exact-match）；`NormalizeTask`（`internal/automation/model.go:223`，调用点 L233，confidence=0.90，exact-match）；`chatTUI::recallSubmittedInput`（`internal/cli/chat_tui.go:544`，调用点 L556，confidence=0.90，exact-match）；`chatTUI::navigateQueue`（`internal/cli/chat_tui.go:585`，调用点 L595，confidence=0.90，exact-match）；`chatTUI::View`（`internal/cli/chat_tui.go:2210`，调用点 L2216，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（10）：** `writeLine`（`internal/automation/prompt.go:126`，调用点 L734/L735/L737/L738，confidence=0.90，exact-match，25 条边）；`Session::Replace`（`internal/agent/session.go:46`，调用点 L899/L914，confidence=0.90，exact-match，2 条边）；`runnerFailedRunResult`（`internal/automation/runner.go:278`，调用点 L163/L171，confidence=0.90，exact-match，2 条边）；`readZipFile`（`internal/automation/xlsx_helpers.go:10`，调用点 L430/L1111，confidence=0.90，exact-match，2 条边）；`NormalizeTask`（`internal/automation/model.go:223`，调用点 L255，confidence=0.90，exact-match）；`failedRunResult`（`internal/automation/runner.go:241`，调用点 L167，confidence=0.90，exact-match）；`parseScheduleClock`（`internal/automation/schedule.go:379`，调用点 L205，confidence=0.90，exact-match）；`xlsxColumnIndex`（`internal/automation/xlsx_helpers.go:25`，调用点 L442，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `ArchiveSink::Emit`（`internal/automation/archive_sink.go:67`，调用点 L188，confidence=0.70，exact-match，待源码复核）；`ArchiveSink::Close`（`internal/automation/archive_sink.go:108`，调用点 L468，confidence=0.70，exact-match，待源码复核）；`BusinessDataKind`（`internal/automation/business.go:28`，调用点 L272，confidence=0.70，exact-match，待源码复核）；`BusinessSourceType`（`internal/automation/business.go:38`，调用点 L273，confidence=0.70，exact-match，待源码复核）；`WriteResult`（`internal/automation/result.go:35`，调用点 L183，confidence=0.70，exact-match，待源码复核）；`Selector::now`（`internal/profile/selector.go:291`，调用点 L174，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 201 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 14 个文件；`internal/automation`、`internal/cli`、`internal/agent`、`internal/fileextract`、`internal/fileutil`、`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`、`internal/automation/business_test.go`
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/business.go`、`internal/cli/chat_tui.go`、`internal/automation/model.go`、`internal/automation/prompt.go`、`internal/agent/session.go`、`internal/automation/runner.go`、`internal/automation/xlsx_helpers.go`、`internal/cli/chat_tui_test.go`、`internal/automation/business_test.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`

#### 待修改项

- [ ] 先冻结 `internal/automation/business.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `businessMetrics`、`WeeklyBusinessDiagnosticsTask`、`businessRecordFromRow`、`businessDiagnosticsMarkdown`、`BusinessDiagnosticsRunner::Run` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-060、CX-061。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/automation/business.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/business.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/automation/business.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G46 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/expert-202607081956`、`origin/expert-202607082134` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`、`internal/automation/business_test.go`。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 race：`go test -race ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/business.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/automation/business.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-060 · 圈复杂度 · `ValidateBusinessDiagnostics`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-060 |
| Baseline key | `complexity\|internal/automation/business.go\|ValidateBusinessDiagnostics` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 30） |
| 文件 | `internal/automation/business.go` |
| 代码行 | 284 |
| 函数/方法/包 | `ValidateBusinessDiagnostics` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ValidateBusinessDiagnostics` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 284-322 行；可见控制流热点：L285 `if !cfg.TargetStart.Before(cfg.TargetEnd) {`；L293 `if source.ID == "" {`；L300 `if !validBusinessDataKind(source.Kind) {`；L307 `case BusinessSourceCSV, BusinessSourceXLSX:`；L311 `case BusinessSourceHTTP:`；L316 `if source.Rows == nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ValidateBusinessDiagnostics（internal/automation/business.go:284-322）
- **可信生产调用方（2）：** `NormalizeBusinessDiagnostics`（`internal/automation/business.go:258`，调用点 L278，confidence=0.90，exact-match）；`ValidateTask`（`internal/automation/model.go:246`，调用点 L266，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `validBusinessDataKind`（`internal/automation/business.go:817`，调用点 L300，confidence=0.90，exact-match）；`validBusinessSourceType`（`internal/automation/business.go:826`，调用点 L303，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `WeeklyBusinessDiagnosticsTask`（`internal/automation/business.go:204`，调用点 L208，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 11 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/business.go`、`internal/automation/model.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 先冻结 `ValidateBusinessDiagnostics` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L285、L293、L306、L311、L316 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `ValidateBusinessDiagnostics` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/business.go|ValidateBusinessDiagnostics` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/business.go:284-322` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ValidateBusinessDiagnostics` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G46 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/expert-202607081956`、`origin/expert-202607082134` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/business.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/business.go|ValidateBusinessDiagnostics` 下降或消失且无新增/恶化。
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
