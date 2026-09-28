# G46 · internal/automation · lease 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G46 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/automation` |
| 涉及文件 | `internal/automation/lease.go`<br>`internal/automation/condition.go`<br>`internal/automation/export.go`<br>`internal/automation/migration_heartbeat.go`<br>`internal/automation/archive.go`<br>`internal/automation/templates.go`<br>`internal/automation/store.go`<br>`internal/automation/business.go`<br>`internal/automation/service.go` |
| 主要治理目标 | 围绕 `internal/automation/store.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | automation 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/main-v2-start-20260630`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/expert-202607081956`、`origin/expert-202607082134` |
| 建议处理顺序 | 总序第 46 组；等待 G28 的同文件变更合并 |
| 前置依赖 | G28（internal/automation/business.go） |
| 后续验证重点 | 返回值、错误语义和现有调用顺序；持久化、迁移或文件格式兼容；权限、安全或凭据边界；配置、平台或向后兼容；会话、状态或恢复语义；协议、序列化或流式事件兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **36/100**，对象 `*Store.AcquireLease` 的分项为严重度 2.5、集中度 3.6、影响面 9.6、可执行性 20.0；最高超限 1.10x，文件内 2 条问题，CodeGraph 记录 1 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 9 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G28。后续同文件组：G49，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-066` | `complexity\|internal/automation/lease.go\|*Store.AcquireLease` | 圈复杂度 | `internal/automation/lease.go` | `*Store.AcquireLease` | 35 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 4 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-067` | `complexity\|internal/automation/lease.go\|*Store.ReleaseLease` | 圈复杂度 | `internal/automation/lease.go` | `*Store.ReleaseLease` | 103 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-062` | `complexity\|internal/automation/condition.go\|Checker.Check` | 圈复杂度 | `internal/automation/condition.go` | `Checker.Check` | 26 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 6 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `CX-065` | `complexity\|internal/automation/export.go\|parseMarkdownExportBlocks` | 圈复杂度 | `internal/automation/export.go` | `parseMarkdownExportBlocks` | 142 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 24 unresolved | 待修改 |
| `CX-068` | `complexity\|internal/automation/migration_heartbeat.go\|legacyHeartbeatCalendarSchedule` | 圈复杂度 | `internal/automation/migration_heartbeat.go` | `legacyHeartbeatCalendarSchedule` | 170 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-059` | `complexity\|internal/automation/archive.go\|pathID` | 圈复杂度 | `internal/automation/archive.go` | `pathID` | 291 | 13/10 | +3（1.30x） | P3 | 2 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 7 unresolved | 待修改 |
| `CX-078` | `complexity\|internal/automation/templates.go\|taskTemplateInterval` | 圈复杂度 | `internal/automation/templates.go` | `taskTemplateInterval` | 192 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-077` | `complexity\|internal/automation/store.go\|*Store.migrateLegacyTaskTemplates` | 圈复杂度 | `internal/automation/store.go` | `*Store.migrateLegacyTaskTemplates` | 346 | 15/10 | +5（1.50x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-061` | `complexity\|internal/automation/business.go\|readBusinessHTTP` | 圈复杂度 | `internal/automation/business.go` | `readBusinessHTTP` | 456 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 4 可信 + 1 待核 callee / 11 unresolved | 待修改 |
| `CX-074` | `complexity\|internal/automation/service.go\|readRunEventSummaries` | 圈复杂度 | `internal/automation/service.go` | `readRunEventSummaries` | 402 | 16/10 | +6（1.60x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 1 待核 callee / 21 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-066 · 圈复杂度 · `*Store.AcquireLease`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-066 |
| Baseline key | `complexity\|internal/automation/lease.go\|*Store.AcquireLease` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/automation/lease.go` |
| 代码行 | 35 |
| 函数/方法/包 | `*Store.AcquireLease` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Store.AcquireLease` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 35-71 行；可见控制流热点：L37 `if err != nil {`；L41 `if err != nil {`；L45 `if err != nil {`；L52 `if err == nil && existing.ExpiresAt.After(now) && (existing.Owner != owner || existing.RunID != run…`；L55 `if err != nil && !errors.Is(err, os.ErrNotExist) {`；L67 `if err := writeLease(path, lease); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeLeaseInput）、持久化/I-O（readLease、writeLease、lockLeaseFile、Store::leasePath）、任务/并发（lockLeaseFile）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::AcquireLease（internal/automation/lease.go:35-71）
- **可信生产调用方（1）：** `Engine::runTask`（`internal/automation/engine.go:161`，调用点 L167，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `normalizeLeaseInput`（`internal/automation/lease.go:148`，调用点 L36，confidence=0.90，exact-match）；`readLease`（`internal/automation/lease.go:161`，调用点 L51，confidence=0.90，exact-match）；`writeLease`（`internal/automation/lease.go:173`，调用点 L67，confidence=0.90，exact-match）；`lockLeaseFile`（`internal/automation/lease.go:185`，调用点 L44，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Store::leasePath`（`internal/automation/lease.go:136`，调用点 L40，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/automation/lease_test.go`、`internal/automation/engine_test.go`、`internal/automation/service_test.go`
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/export_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/lease.go`、`internal/automation/engine.go`、`internal/automation/lease_test.go`、`internal/automation/engine_test.go`、`internal/automation/service_test.go`、`internal/automation/architecture_test.go`

#### 待修改项

- [ ] 逐段核对 L37、L41、L52、L55、L67 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Store.AcquireLease` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/lease.go|*Store.AcquireLease` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/lease.go:35-71` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Store.AcquireLease` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/automation/lease_test.go`、`internal/automation/engine_test.go`、`internal/automation/service_test.go`。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/lease.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/lease.go|*Store.AcquireLease` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-067 · 圈复杂度 · `*Store.ReleaseLease`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-067 |
| Baseline key | `complexity\|internal/automation/lease.go\|*Store.ReleaseLease` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/automation/lease.go` |
| 代码行 | 103 |
| 函数/方法/包 | `*Store.ReleaseLease` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Store.ReleaseLease` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 103-134 行；可见控制流热点：L107 `if taskID == "" || runID == "" || owner == "" {`；L111 `if err != nil {`；L115 `if err != nil {`；L124 `if err != nil {`；L127 `if lease.Owner != owner || lease.RunID != runID {`；L130 `if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readLease、lockLeaseFile、Store::leasePath）、任务/并发（lockLeaseFile）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::ReleaseLease（internal/automation/lease.go:103-134）
- **可信生产调用方（1）：** `Engine::runTask`（`internal/automation/engine.go:161`，调用点 L170，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `readLease`（`internal/automation/lease.go:161`，调用点 L120，confidence=0.90，exact-match）；`lockLeaseFile`（`internal/automation/lease.go:185`，调用点 L114，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Store::leasePath`（`internal/automation/lease.go:136`，调用点 L110，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/automation/lease_test.go`
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/lease.go`、`internal/automation/engine.go`、`internal/automation/lease_test.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`

#### 待修改项

- [ ] 逐段核对 L107、L115、L121、L127、L130 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Store.ReleaseLease` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/lease.go|*Store.ReleaseLease` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/lease.go:103-134` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Store.ReleaseLease` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/automation/lease_test.go`。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/lease.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/lease.go|*Store.ReleaseLease` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-062 · 圈复杂度 · `Checker.Check`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-062 |
| Baseline key | `complexity\|internal/automation/condition.go\|Checker.Check` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/automation/condition.go` |
| 代码行 | 26 |
| 函数/方法/包 | `Checker.Check` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Checker.Check` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 26-60 行；可见控制流热点：L28 `if err := ctx.Err(); err != nil {`；L34 `if !due.Due {`；L37 `if err := workspaceExists(task.WorkspaceRoot); err != nil {`；L43 `if blocked, reason, err := c.failureCooldown(task, now); err != nil {`；L49 `if prompt == "" {`；L56 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseConditionPromptResult、Checker::resolveModel）、持久化/I-O（ArchiveSink::Err）、权限/安全（conditionAllowed）、任务/并发（conditionBlocked）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Checker::Check（internal/automation/condition.go:26-60）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Engine::runTask`（`internal/automation/engine.go:161`，调用点 L204，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（6）：** `conditionBlocked`（`internal/automation/condition.go:158`，调用点 L29/L38/L41/L44，confidence=0.90，exact-match，6 条边）；`conditionSkipped`（`internal/automation/condition.go:154`，调用点 L32/L35/L46，confidence=0.90，exact-match，3 条边）；`parseConditionPromptResult`（`internal/automation/condition.go:101`，调用点 L59，confidence=0.90，exact-match）；`workspaceExists`（`internal/automation/condition.go:135`，调用点 L37，confidence=0.90，exact-match）；`conditionAllowed`（`internal/automation/condition.go:150`，调用点 L50，confidence=0.90，exact-match）；`defaultString`（`internal/automation/schedule.go:433`，调用点 L35，confidence=0.90，exact-match）
- **待源码复核被调候选（4）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L28，confidence=0.70，instance-method，待源码复核）；`ConditionModel::Decide`（`internal/automation/condition.go:16`，调用点 L55，confidence=0.70，exact-match，待源码复核）；`Checker::resolveModel`（`internal/automation/condition.go:62`，调用点 L40，confidence=0.70，instance-method，待源码复核）；`Checker::failureCooldown`（`internal/automation/condition.go:69`，调用点 L43，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/condition.go`、`internal/automation/engine.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 逐段核对 L28、L34、L40、L49、L56 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、权限/安全 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Checker.Check` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/condition.go|Checker.Check` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/condition.go:26-60` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Checker.Check` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：持久化、迁移或文件格式兼容；权限、安全或凭据边界。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/condition.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/condition.go|Checker.Check` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-065 · 圈复杂度 · `parseMarkdownExportBlocks`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-065 |
| Baseline key | `complexity\|internal/automation/export.go\|parseMarkdownExportBlocks` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/automation/export.go` |
| 代码行 | 142 |
| 函数/方法/包 | `parseMarkdownExportBlocks` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`parseMarkdownExportBlocks` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 142-188 行；可见控制流热点：L147 `if len(paragraph) == 0 {`；L156 `for _, raw := range lines {`；L158 `if line == "" {`；L166 `if level, text, ok := parseMarkdownHeading(line); ok {`；L171 `if text, ok := parseMarkdownListItem(line); ok {`；L184 `if len(blocks) == 0 && strings.TrimSpace(markdown) != "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseMarkdownHeading、parseMarkdownListItem）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function parseMarkdownExportBlocks（internal/automation/export.go:142-188）
- **可信生产调用方（1）：** `buildRunResultExportPayload`（`internal/automation/export.go:113`，调用点 L114，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `stripInlineMarkdown`（`internal/automation/export.go:230`，调用点 L150/L168/L173/L178，confidence=0.90，exact-match，5 条边）；`parseMarkdownHeading`（`internal/automation/export.go:190`，调用点 L166，confidence=0.90，exact-match）；`parseMarkdownListItem`（`internal/automation/export.go:201`，调用点 L171，confidence=0.90，exact-match）；`isMarkdownRule`（`internal/automation/export.go:218`，调用点 L162，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `service::ExportRunResult`（`internal/automation/export.go:20`，调用点 L47，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 24 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/export.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 逐段核对 L147、L156、L162、L171、L184 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `parseMarkdownExportBlocks` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/export.go|parseMarkdownExportBlocks` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/export.go:142-188` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `parseMarkdownExportBlocks` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/export.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/export.go|parseMarkdownExportBlocks` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-068 · 圈复杂度 · `legacyHeartbeatCalendarSchedule`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-068 |
| Baseline key | `complexity\|internal/automation/migration_heartbeat.go\|legacyHeartbeatCalendarSchedule` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/automation/migration_heartbeat.go` |
| 代码行 | 170 |
| 函数/方法/包 | `legacyHeartbeatCalendarSchedule` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`legacyHeartbeatCalendarSchedule` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 170-219 行；可见控制流热点：L172 `if idx < 0 {`；L180 `if parts := strings.SplitN(raw, "@", 2); len(parts) == 2 {`；L189 `if parts := strings.SplitN(raw, ":", 2); len(parts) == 2 {`；L196 `case "daily":`；L199 `if len(weekdays) == 0 {`；L205 `case "yearly":`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseScheduleClock）、任务/并发（parseScheduleClock）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function legacyHeartbeatCalendarSchedule（internal/automation/migration_heartbeat.go:170-219）
- **可信生产调用方（1）：** `ConvertHeartbeatSchedule`（`internal/automation/migration_heartbeat.go:150`，调用点 L155，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `legacyHeartbeatWeekdays`（`internal/automation/migration_heartbeat.go:253`，调用点 L198，confidence=0.90，exact-match）；`legacyHeartbeatYearly`（`internal/automation/migration_heartbeat.go:284`，调用点 L206，confidence=0.90，exact-match）；`legacyPositiveInt`（`internal/automation/migration_heartbeat.go:300`，调用点 L204，confidence=0.90，exact-match）；`parseScheduleClock`（`internal/automation/schedule.go:379`，调用点 L184，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/migration_heartbeat.go`、`internal/automation/schedule.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 逐段核对 L172、L184、L195、L199、L205 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `legacyHeartbeatCalendarSchedule` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/migration_heartbeat.go|legacyHeartbeatCalendarSchedule` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/migration_heartbeat.go:170-219` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `legacyHeartbeatCalendarSchedule` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/migration_heartbeat.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/migration_heartbeat.go|legacyHeartbeatCalendarSchedule` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-059 · 圈复杂度 · `pathID`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-059 |
| Baseline key | `complexity\|internal/automation/archive.go\|pathID` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/automation/archive.go` |
| 代码行 | 291 |
| 函数/方法/包 | `pathID` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`pathID` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 291-309 行；可见控制流热点：L293 `if id == "" {`；L296 `if id == "." || id == ".." {`；L300 `for i := 0; i < len(id); i++ {`；L302 `if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '…`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function pathID（internal/automation/archive.go:291-309）
- **可信生产调用方（2）：** `Store::RunPaths`（`internal/automation/archive.go:62`，调用点 L63/L67，confidence=0.90，exact-match，2 条边）；`Store::ListRuns`（`internal/automation/archive.go:156`，调用点 L157，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Store::WriteResult`（`internal/automation/archive.go:193`，调用点 L194，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 7 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/archive.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 逐段核对 L293、L296、L300、L302 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `pathID` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/archive.go|pathID` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/archive.go:291-309` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `pathID` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/archive.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/archive.go|pathID` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-078 · 圈复杂度 · `taskTemplateInterval`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-078 |
| Baseline key | `complexity\|internal/automation/templates.go\|taskTemplateInterval` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 32） |
| 文件 | `internal/automation/templates.go` |
| 代码行 | 192 |
| 函数/方法/包 | `taskTemplateInterval` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`taskTemplateInterval` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 192-227 行；可见控制流热点：L193 `switch normalizeScheduleKind(schedule.Kind) {`；L195 `if !schedule.RunAt.IsZero() {`；L201 `if timeOfDay == "" {`；L207 `if days == "" {`；L214 `case "yearly":`；L222 `if every := strings.TrimSpace(schedule.Every); every != "" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function taskTemplateInterval（internal/automation/templates.go:192-227）
- **可信生产调用方（1）：** `TaskTemplateFromTask`（`internal/automation/templates.go:140`，调用点 L152，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `intsCSV`（`internal/automation/templates.go:229`，调用点 L212/L215/L216，confidence=0.90，exact-match，3 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/templates.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 逐段核对 L193、L198、L205、L211、L222 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `taskTemplateInterval` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/templates.go|taskTemplateInterval` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/templates.go:192-227` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `taskTemplateInterval` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/templates.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/templates.go|taskTemplateInterval` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-077 · 圈复杂度 · `*Store.migrateLegacyTaskTemplates`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-077 |
| Baseline key | `complexity\|internal/automation/store.go\|*Store.migrateLegacyTaskTemplates` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 30） |
| 文件 | `internal/automation/store.go` |
| 代码行 | 346 |
| 函数/方法/包 | `*Store.migrateLegacyTaskTemplates` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Store.migrateLegacyTaskTemplates` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 346-397 行；可见控制流热点：L348 `if err := db.QueryRow(\`SELECT count(*) FROM task_template_migrations WHERE name = ?\`, templatesMigr…`；L357 `if err != nil {`；L365 `if len(bytes.TrimSpace(b)) == 0 {`；L377 `if err != nil {`；L385 `for _, template := range templates {`；L393 `if err := tx.Commit(); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeTaskTemplates）、持久化/I-O（saveTaskTemplateRow、Store::templatesPath、Store::Begin）、任务/并发（saveTaskTemplateRow、normalizeTaskTemplates）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::migrateLegacyTaskTemplates（internal/automation/store.go:346-397）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Store::ensureTemplatesDB`（`internal/automation/store.go:336`，调用点 L343，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `saveTaskTemplateRow`（`internal/automation/store.go:399`，调用点 L386，confidence=0.90，exact-match）；`normalizeTaskTemplates`（`internal/automation/store.go:422`，调用点 L376，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Store::templatesPath`（`internal/automation/store.go:296`，调用点 L354/L363/L374，confidence=0.70，instance-method，3 条边，待源码复核）；`Store::Begin`（`internal/checkpoint/checkpoint.go:102`，调用点 L380，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Store::SaveTaskTemplate`（`internal/automation/store.go:130`，调用点 L140，confidence=0.70，instance-method，待源码复核）；`Store::DeleteTaskTemplate`（`internal/automation/store.go:158`，调用点 L168，confidence=0.70，instance-method，待源码复核）；`Store::RestoreTaskTemplate`（`internal/automation/store.go:212`，调用点 L219，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/automation`、`internal/checkpoint`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/store.go`、`internal/checkpoint/checkpoint.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 先冻结 `*Store.migrateLegacyTaskTemplates` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L348、L357、L373、L385、L393 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Store.migrateLegacyTaskTemplates` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/store.go|*Store.migrateLegacyTaskTemplates` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/store.go:346-397` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Store.migrateLegacyTaskTemplates` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/main-v2-start-20260630` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/store.go|*Store.migrateLegacyTaskTemplates` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-061 · 圈复杂度 · `readBusinessHTTP`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-061 |
| Baseline key | `complexity\|internal/automation/business.go\|readBusinessHTTP` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 29） |
| 文件 | `internal/automation/business.go` |
| 代码行 | 456 |
| 函数/方法/包 | `readBusinessHTTP` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`readBusinessHTTP` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 456-495 行；可见控制流热点：L457 `if client == nil {`；L465 `if err != nil {`；L473 `if err != nil {`；L478 `case "csv":`；L482 `if err := json.Unmarshal(body, &rows); err != nil {`；L488 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readBusinessCSV、readBusinessXLSX、ArchiveSink::Close）、渲染/输出（inferBusinessFormat）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function readBusinessHTTP（internal/automation/business.go:456-495）
- **可信生产调用方（1）：** `readBusinessSource`（`internal/automation/business.go:356`，调用点 L374，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `readBusinessCSV`（`internal/automation/business.go:398`，调用点 L479，confidence=0.90，exact-match）；`readBusinessXLSX`（`internal/automation/business.go:425`，调用点 L491，confidence=0.90，exact-match）；`rowsFromBusinessMaps`（`internal/automation/business.go:497`，调用点 L485，confidence=0.90，exact-match）；`inferBusinessFormat`（`internal/automation/business.go:1040`，调用点 L476，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `ArchiveSink::Close`（`internal/automation/archive_sink.go:108`，调用点 L468，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 11 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/business.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`

#### 待修改项

- [ ] 先冻结 `readBusinessHTTP` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L457、L469、L477、L482、L488 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `readBusinessHTTP` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/business.go|readBusinessHTTP` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/business.go:456-495` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `readBusinessHTTP` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/expert-202607081956`、`origin/expert-202607082134` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/business.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/business.go|readBusinessHTTP` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-074 · 圈复杂度 · `readRunEventSummaries`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-074 |
| Baseline key | `complexity\|internal/automation/service.go\|readRunEventSummaries` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 29） |
| 文件 | `internal/automation/service.go` |
| 代码行 | 402 |
| 函数/方法/包 | `readRunEventSummaries` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`readRunEventSummaries` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 402-469 行；可见控制流热点：L403 `if limit <= 0 {`；L410 `if err != nil {`；L424 `if len(events) < limit {`；L438 `if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {`；L449 `if stream != nil && stream.Kind == raw.Kind && stream.Level == raw.Level {`；L465 `if err := scanner.Err(); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（previewRunes、compactRunEventKind、appendRunEventText）、渲染/输出（previewRunes）、网络/协议（compactRunEventKind、appendRunEventText）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function readRunEventSummaries（internal/automation/service.go:402-469）
- **可信生产调用方（1）：** `service::ReadRun`（`internal/automation/service.go:331`，调用点 L353，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `previewRunes`（`internal/automation/service.go:516`，调用点 L423/L459，confidence=0.90，exact-match，2 条边）；`compactRunEventKind`（`internal/automation/service.go:471`，调用点 L448，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `appendRunEventText`（`internal/automation/service.go:480`，调用点 L450，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `service::ReadRun`（`internal/automation/service.go:331`，调用点 L353，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 21 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/automation/service_test.go`
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/service.go`、`internal/automation/service_test.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`

#### 待修改项

- [ ] 先冻结 `readRunEventSummaries` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L403、L420、L429、L449、L465 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、渲染/输出、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `readRunEventSummaries` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/service.go|readRunEventSummaries` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/service.go:402-469` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `readRunEventSummaries` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用中 `origin/codex/main-v2-start-20260630` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/automation/service_test.go`。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/service.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/service.go|readRunEventSummaries` 下降或消失且无新增/恶化。
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
