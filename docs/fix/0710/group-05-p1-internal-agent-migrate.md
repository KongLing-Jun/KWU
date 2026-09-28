# G05 · internal/agent · migrate 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G05 |
| 问题数量 | 10 |
| 整体优先级 | P1 |
| 涉及模块 | `internal/agent` |
| 涉及文件 | `internal/agent/migrate.go`<br>`internal/agent/parallel_tasks.go` |
| 主要治理目标 | 围绕 `internal/agent/migrate.go` 等治理对象降低复杂度/体量，统一处理 2 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | agent 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 12 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 5 组；等待 G01 的同文件变更合并 |
| 前置依赖 | G01（internal/agent/migrate.go） |
| 后续验证重点 | 持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容；并发、取消或事件顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **62/100**，对象 `migrateSubDirectory` 的分项为严重度 13.8、集中度 18.4、影响面 22.1、可执行性 8.0；最高超限 2.90x，文件内 12 条问题，CodeGraph 记录 1 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数、嵌套深度，必须合并治理。

### 治理边界与组间关系

本组涉及 2 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G01。后续同文件组：G25，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-035` | `complexity\|internal/agent/migrate.go\|migrateSubDirectory` | 圈复杂度 | `internal/agent/migrate.go` | `migrateSubDirectory` | 352 | 29/10 | +19（2.90x） | P1 | 1 可信 + 0 待核 caller / 5 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `FN-007` | `func_lines\|internal/agent/migrate.go\|migrateSubDirectory` | 函数有效行数 | `internal/agent/migrate.go` | `migrateSubDirectory` | 352 | 84/80 | +4（1.05x） | P1 | 1 可信 + 0 待核 caller / 5 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `ND-004` | `nesting_depth\|internal/agent/migrate.go\|migrateSubDirectory` | 嵌套深度 | `internal/agent/migrate.go` | `migrateSubDirectory` | 352 | 5/4 | +1（1.25x） | P1 | 1 可信 + 0 待核 caller / 5 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-040` | `complexity\|internal/agent/parallel_tasks.go\|*ParallelTasksTool.Execute` | 圈复杂度 | `internal/agent/parallel_tasks.go` | `*ParallelTasksTool.Execute` | 111 | 48/10 | +38（4.80x） | P1 | 0 可信 + 1 待核 caller / 10 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `FN-008` | `func_lines\|internal/agent/parallel_tasks.go\|*ParallelTasksTool.Execute` | 函数有效行数 | `internal/agent/parallel_tasks.go` | `*ParallelTasksTool.Execute` | 111 | 229/80 | +149（2.86x） | P1 | 0 可信 + 1 待核 caller / 10 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `CX-036` | `complexity\|internal/agent/migrate.go\|transformAndCopyJsonl` | 圈复杂度 | `internal/agent/migrate.go` | `transformAndCopyJsonl` | 482 | 23/10 | +13（2.30x） | P2 | 4 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-033` | `complexity\|internal/agent/migrate.go\|importJsonlSessions` | 圈复杂度 | `internal/agent/migrate.go` | `importJsonlSessions` | 271 | 16/10 | +6（1.60x） | P2 | 1 可信 + 0 待核 caller / 6 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-034` | `complexity\|internal/agent/migrate.go\|jsonlSessionDestDir` | 圈复杂度 | `internal/agent/migrate.go` | `jsonlSessionDestDir` | 323 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-038` | `complexity\|internal/agent/migrate.go\|strandedSessionDestDir` | 圈复杂度 | `internal/agent/migrate.go` | `strandedSessionDestDir` | 756 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-039` | `complexity\|internal/agent/migrate.go\|copyFileIfExists` | 圈复杂度 | `internal/agent/migrate.go` | `copyFileIfExists` | 830 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-035 · 圈复杂度 · `migrateSubDirectory`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-035 |
| Baseline key | `complexity\|internal/agent/migrate.go\|migrateSubDirectory` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P1，治理对象得分 62） |
| 文件 | `internal/agent/migrate.go` |
| 代码行 | 352 |
| 函数/方法/包 | `migrateSubDirectory` |
| Baseline / 当前复测 / 阈值 | 29 / 29 / 10 |
| 超出 | +19，2.90x |
| 同一治理对象关联问题 | `FN-007`、`ND-004` |

#### 明确的问题描述

`migrateSubDirectory` 的圈复杂度当前为 **29**，超过限制 **10**（+19，2.90x）。定义范围为 352-439 行；可见控制流热点：L354 `if err != nil {`；L367 `if e.IsDir() {`；L380 `if jsonlInfo, err := os.Stat(jsonlPath); err == nil {`；L399 `if !isMessageFormat(srcPath) {`；L418 `if reconstruct {`；L432 `if srcInfo != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readLegacyMeta、fileExists）、渲染/输出（isMessageFormat）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function migrateSubDirectory（internal/agent/migrate.go:352-439）
- **可信生产调用方（1）：** `migrateLegacySessions`（`internal/agent/migrate.go:82`，调用点 L252，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `isMessageFormat`（`internal/agent/migrate.go:444`，调用点 L378/L399，confidence=0.90，exact-match，2 条边）；`transformAndCopyJsonl`（`internal/agent/migrate.go:482`，调用点 L428，confidence=0.90，exact-match）；`readLegacyMeta`（`internal/agent/migrate.go:572`，调用点 L406，confidence=0.90，exact-match）；`recordImportedTitle`（`internal/agent/migrate.go:613`，调用点 L435，confidence=0.90，exact-match）；`reconstructSession`（`internal/agent/migrate.go:892`，调用点 L419，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `fileExists`（`internal/agent/migrate.go:456`，调用点 L378，confidence=0.70，exact-match，待源码复核）；`dirExists`（`internal/agent/migrate.go:584`，调用点 L408，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `MigrateLegacySessions`（`internal/agent/migrate.go:70`，调用点 L71，confidence=0.90，exact-match，待源码复核）；`MigrateLegacySessionsFromConfigDir`（`internal/agent/migrate.go:78`，调用点 L79，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/migrate.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `migrateSubDirectory` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L354、L373、L393、L414、L432 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `migrateSubDirectory` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-035、FN-007、ND-004 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/migrate.go|migrateSubDirectory` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/migrate.go:352-439` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `migrateSubDirectory` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G25 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-035`、`FN-007`、`ND-004` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/migrate.go|migrateSubDirectory` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-007 · 函数有效行数 · `migrateSubDirectory`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-007 |
| Baseline key | `func_lines\|internal/agent/migrate.go\|migrateSubDirectory` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P3，治理对象得分 62） |
| 文件 | `internal/agent/migrate.go` |
| 代码行 | 352 |
| 函数/方法/包 | `migrateSubDirectory` |
| Baseline / 当前复测 / 阈值 | 84 / 84 / 80 |
| 超出 | +4，1.05x |
| 同一治理对象关联问题 | `CX-035`、`ND-004` |

#### 明确的问题描述

`migrateSubDirectory` 的有效代码行 **84**，超过限制 **80**（+4）。CodeGraph 定义物理范围为 352-439 行，并记录 7 个直接下游候选；候选职责分布为 持久化/I-O（readLegacyMeta、fileExists）、渲染/输出（isMessageFormat）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function migrateSubDirectory（internal/agent/migrate.go:352-439）
- **可信生产调用方（1）：** `migrateLegacySessions`（`internal/agent/migrate.go:82`，调用点 L252，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `isMessageFormat`（`internal/agent/migrate.go:444`，调用点 L378/L399，confidence=0.90，exact-match，2 条边）；`transformAndCopyJsonl`（`internal/agent/migrate.go:482`，调用点 L428，confidence=0.90，exact-match）；`readLegacyMeta`（`internal/agent/migrate.go:572`，调用点 L406，confidence=0.90，exact-match）；`recordImportedTitle`（`internal/agent/migrate.go:613`，调用点 L435，confidence=0.90，exact-match）；`reconstructSession`（`internal/agent/migrate.go:892`，调用点 L419，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `fileExists`（`internal/agent/migrate.go:456`，调用点 L378，confidence=0.70，exact-match，待源码复核）；`dirExists`（`internal/agent/migrate.go:584`，调用点 L408，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `MigrateLegacySessions`（`internal/agent/migrate.go:70`，调用点 L71，confidence=0.90，exact-match，待源码复核）；`MigrateLegacySessionsFromConfigDir`（`internal/agent/migrate.go:78`，调用点 L79，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/migrate.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `migrateSubDirectory` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `migrateSubDirectory` 的 84 个有效行按 持久化/I-O、渲染/输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L377 `Prefer .jsonl sidecar if it's newer.`、L402 `reconstruct stays false` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-035、FN-007、ND-004 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/agent/migrate.go|migrateSubDirectory` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/migrate.go:352-439` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `migrateSubDirectory` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G25 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-035`、`FN-007`、`ND-004` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/agent/migrate.go|migrateSubDirectory` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-004 · 嵌套深度 · `migrateSubDirectory`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-004 |
| Baseline key | `nesting_depth\|internal/agent/migrate.go\|migrateSubDirectory` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P3，治理对象得分 62） |
| 文件 | `internal/agent/migrate.go` |
| 代码行 | 352 |
| 函数/方法/包 | `migrateSubDirectory` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-035`、`FN-007` |

#### 明确的问题描述

`migrateSubDirectory` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L354 `if err != nil {`；L367 `if e.IsDir() {`；L380 `if jsonlInfo, err := os.Stat(jsonlPath); err == nil {`；L399 `if !isMessageFormat(srcPath) {`；L418 `if reconstruct {`；L432 `if srcInfo != nil {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function migrateSubDirectory（internal/agent/migrate.go:352-439）
- **可信生产调用方（1）：** `migrateLegacySessions`（`internal/agent/migrate.go:82`，调用点 L252，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `isMessageFormat`（`internal/agent/migrate.go:444`，调用点 L378/L399，confidence=0.90，exact-match，2 条边）；`transformAndCopyJsonl`（`internal/agent/migrate.go:482`，调用点 L428，confidence=0.90，exact-match）；`readLegacyMeta`（`internal/agent/migrate.go:572`，调用点 L406，confidence=0.90，exact-match）；`recordImportedTitle`（`internal/agent/migrate.go:613`，调用点 L435，confidence=0.90，exact-match）；`reconstructSession`（`internal/agent/migrate.go:892`，调用点 L419，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `fileExists`（`internal/agent/migrate.go:456`，调用点 L378，confidence=0.70，exact-match，待源码复核）；`dirExists`（`internal/agent/migrate.go:584`，调用点 L408，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `MigrateLegacySessions`（`internal/agent/migrate.go:70`，调用点 L71，confidence=0.90，exact-match，待源码复核）；`MigrateLegacySessionsFromConfigDir`（`internal/agent/migrate.go:78`，调用点 L79，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/migrate.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `migrateSubDirectory` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 从 L354、L373、L393、L414、L432 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-035、FN-007、ND-004 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/agent/migrate.go|migrateSubDirectory` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/migrate.go:352-439` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `migrateSubDirectory` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G25 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-035`、`FN-007`、`ND-004` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/agent/migrate.go|migrateSubDirectory` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-040 · 圈复杂度 · `*ParallelTasksTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-040 |
| Baseline key | `complexity\|internal/agent/parallel_tasks.go\|*ParallelTasksTool.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 62） |
| 文件 | `internal/agent/parallel_tasks.go` |
| 代码行 | 111 |
| 函数/方法/包 | `*ParallelTasksTool.Execute` |
| Baseline / 当前复测 / 阈值 | 48 / 48 / 10 |
| 超出 | +38，4.80x |
| 同一治理对象关联问题 | `FN-008` |

#### 明确的问题描述

`*ParallelTasksTool.Execute` 的圈复杂度当前为 **48**，超过限制 **10**（+38，4.80x）。定义范围为 111-357 行；可见控制流热点：L115 `if err := json.Unmarshal(args, &params); err != nil {`；L166 `if len(entries) > 0 {`；L250 `if runErr != nil {`；L299 `if done[r.index] {`；L329 `for completed < n {`；L351 `if err == nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（validateParallelTaskItems、resolveSubagentProvider）、状态与控制（RunSubAgentWithSession、ParallelTasksTool::runAsBackgroundJobs）、持久化/I-O（TaskTool::effectiveProfile、ArchiveSink::Err）、渲染/输出（formatParallelTasksAggregate、Sink::Emit）、网络/协议（resolveSubagentProvider）、任务/并发（formatParallelTasksAggregate、parallelTasksWereCancelled、validateParallelTaskItems、TaskTool::effectiveProfile）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method ParallelTasksTool::Execute（internal/agent/parallel_tasks.go:111-357）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `ParallelTasksTool::runAsBackgroundJobs`（`internal/agent/parallel_tasks.go:409`，调用点 L459，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（10）：** `formatParallelTasksAggregate`（`internal/agent/parallel_tasks.go:368`，调用点 L345/L354/L356，confidence=0.90，exact-match，3 条边）；`CallContext`（`internal/agent/agent.go:88`，调用点 L128，confidence=0.90，exact-match）；`parallelTasksWereCancelled`（`internal/agent/parallel_tasks.go:359`，调用点 L349，confidence=0.90，exact-match）；`validateParallelTaskItems`（`internal/agent/parallel_tasks.go:497`，调用点 L124，confidence=0.90，exact-match）；`resolveSubagentProvider`（`internal/agent/parallel_tasks.go:540`，调用点 L221，confidence=0.90，exact-match）；`TaskTool::effectiveProfile`（`internal/agent/task.go:422`，调用点 L213，confidence=0.90，exact-match）；`TaskTool::buildSubReg`（`internal/agent/task.go:618`，调用点 L214，confidence=0.90，exact-match）；`RunSubAgentWithSession`（`internal/agent/task.go:804`，调用点 L232，confidence=0.90，exact-match）
- **待源码复核被调候选（4）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L247/L248/L271/L334，confidence=0.70，instance-method，5 条边，待源码复核）；`Sink::Emit`（`internal/event/event.go:362`，调用点 L200/L223/L255/L262，confidence=0.80，instance-method，4 条边，待源码复核）；`ParallelTasksTool::runAsBackgroundJobs`（`internal/agent/parallel_tasks.go:409`，调用点 L131，confidence=0.70，instance-method，待源码复核）；`Manager::Wait`（`internal/jobs/jobs.go:702`，调用点 L348，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 7 个文件；`internal/agent`、`internal/event`、`internal/automation`、`internal/jobs`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/parallel_tasks.go`、`internal/agent/agent.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*ParallelTasksTool.Execute` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L115、L184、L275、L319、L351 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*ParallelTasksTool.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-040、FN-008 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/parallel_tasks.go|*ParallelTasksTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/parallel_tasks.go:111-357` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*ParallelTasksTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G25 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-040`、`FN-008` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/parallel_tasks.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/parallel_tasks.go|*ParallelTasksTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-008 · 函数有效行数 · `*ParallelTasksTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-008 |
| Baseline key | `func_lines\|internal/agent/parallel_tasks.go\|*ParallelTasksTool.Execute` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P1，治理对象得分 62） |
| 文件 | `internal/agent/parallel_tasks.go` |
| 代码行 | 111 |
| 函数/方法/包 | `*ParallelTasksTool.Execute` |
| Baseline / 当前复测 / 阈值 | 229 / 229 / 80 |
| 超出 | +149，2.86x |
| 同一治理对象关联问题 | `CX-040` |

#### 明确的问题描述

`*ParallelTasksTool.Execute` 的有效代码行 **229**，超过限制 **80**（+149）。CodeGraph 定义物理范围为 111-357 行，并记录 14 个直接下游候选；候选职责分布为 配置/解析（validateParallelTaskItems、resolveSubagentProvider）、状态与控制（RunSubAgentWithSession、ParallelTasksTool::runAsBackgroundJobs）、持久化/I-O（TaskTool::effectiveProfile、ArchiveSink::Err）、渲染/输出（formatParallelTasksAggregate、Sink::Emit）、网络/协议（resolveSubagentProvider）、任务/并发（formatParallelTasksAggregate、parallelTasksWereCancelled、validateParallelTaskItems、TaskTool::effectiveProfile）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method ParallelTasksTool::Execute（internal/agent/parallel_tasks.go:111-357）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `ParallelTasksTool::runAsBackgroundJobs`（`internal/agent/parallel_tasks.go:409`，调用点 L459，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（10）：** `formatParallelTasksAggregate`（`internal/agent/parallel_tasks.go:368`，调用点 L345/L354/L356，confidence=0.90，exact-match，3 条边）；`CallContext`（`internal/agent/agent.go:88`，调用点 L128，confidence=0.90，exact-match）；`parallelTasksWereCancelled`（`internal/agent/parallel_tasks.go:359`，调用点 L349，confidence=0.90，exact-match）；`validateParallelTaskItems`（`internal/agent/parallel_tasks.go:497`，调用点 L124，confidence=0.90，exact-match）；`resolveSubagentProvider`（`internal/agent/parallel_tasks.go:540`，调用点 L221，confidence=0.90，exact-match）；`TaskTool::effectiveProfile`（`internal/agent/task.go:422`，调用点 L213，confidence=0.90，exact-match）；`TaskTool::buildSubReg`（`internal/agent/task.go:618`，调用点 L214，confidence=0.90，exact-match）；`RunSubAgentWithSession`（`internal/agent/task.go:804`，调用点 L232，confidence=0.90，exact-match）
- **待源码复核被调候选（4）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L247/L248/L271/L334，confidence=0.70，instance-method，5 条边，待源码复核）；`Sink::Emit`（`internal/event/event.go:362`，调用点 L200/L223/L255/L262，confidence=0.80，instance-method，4 条边，待源码复核）；`ParallelTasksTool::runAsBackgroundJobs`（`internal/agent/parallel_tasks.go:409`，调用点 L131，confidence=0.70，instance-method，待源码复核）；`Manager::Wait`（`internal/jobs/jobs.go:702`，调用点 L348，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 7 个文件；`internal/agent`、`internal/event`、`internal/automation`、`internal/jobs`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/parallel_tasks.go`、`internal/agent/agent.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*ParallelTasksTool.Execute` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*ParallelTasksTool.Execute` 的 229 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L130 `Fallback: no event sink available, use background-jobs approach.`、L143 `Dependency state: remaining = number of deps not yet completed.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-040、FN-008 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/agent/parallel_tasks.go|*ParallelTasksTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/parallel_tasks.go:111-357` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*ParallelTasksTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G25 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-040`、`FN-008` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 race：`go test -race ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/parallel_tasks.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/agent/parallel_tasks.go|*ParallelTasksTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-036 · 圈复杂度 · `transformAndCopyJsonl`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-036 |
| Baseline key | `complexity\|internal/agent/migrate.go\|transformAndCopyJsonl` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 47） |
| 文件 | `internal/agent/migrate.go` |
| 代码行 | 482 |
| 函数/方法/包 | `transformAndCopyJsonl` |
| Baseline / 当前复测 / 阈值 | 23 / 23 / 10 |
| 超出 | +13，2.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`transformAndCopyJsonl` 的圈复杂度当前为 **23**，超过限制 **10**（+13，2.30x）。定义范围为 482-568 行；可见控制流热点：L484 `if err != nil {`；L504 `for {`；L514 `if err := json.Unmarshal(raw, &m); err != nil || m.Role != "assistant" || len(m.ToolCalls) == 0 {`；L531 `for i, tc := range legacyCalls {`；L549 `if err != nil {`；L563 `if err := os.Rename(tmpPath, dst); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function transformAndCopyJsonl（internal/agent/migrate.go:482-568）
- **可信生产调用方（4）：** `migrateLegacySessions`（`internal/agent/migrate.go:82`，调用点 L149/L215，confidence=0.90，exact-match，2 条边）；`importJsonlSessions`（`internal/agent/migrate.go:271`，调用点 L305，confidence=0.90，exact-match）；`migrateSubDirectory`（`internal/agent/migrate.go:352`，调用点 L428，confidence=0.90，exact-match）；`rehomeStrandedSessions`（`internal/agent/migrate.go:685`，调用点 L731，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `MigrateLegacySessions`（`internal/agent/migrate.go:70`，调用点 L71，confidence=0.90，exact-match，待源码复核）；`MigrateLegacySessionsFromConfigDir`（`internal/agent/migrate.go:78`，调用点 L79，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/migrate.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `transformAndCopyJsonl` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L484、L506、L523、L549、L563 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `transformAndCopyJsonl` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/migrate.go|transformAndCopyJsonl` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/migrate.go:482-568` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `transformAndCopyJsonl` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G25 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/migrate.go|transformAndCopyJsonl` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-033 · 圈复杂度 · `importJsonlSessions`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-033 |
| Baseline key | `complexity\|internal/agent/migrate.go\|importJsonlSessions` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 45） |
| 文件 | `internal/agent/migrate.go` |
| 代码行 | 271 |
| 函数/方法/包 | `importJsonlSessions` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`importJsonlSessions` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 271-321 行；可见控制流热点：L274 `for _, e := range entries {`；L280 `if hasEvents[base] {`；L291 `if !isMessageFormat(jsonlPath) {`；L298 `if err := copySubagentArtifacts(srcDir, destDir, base); err != nil {`；L308 `if srcInfo != nil {`；L313 `if err := copySubagentArtifacts(srcDir, destDir, base); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 渲染/输出（isMessageFormat）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function importJsonlSessions（internal/agent/migrate.go:271-321）
- **可信生产调用方（1）：** `migrateLegacySessions`（`internal/agent/migrate.go:82`，调用点 L181，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `copySubagentArtifacts`（`internal/agent/migrate.go:810`，调用点 L298/L313，confidence=0.90，exact-match，2 条边）；`jsonlSessionDestDir`（`internal/agent/migrate.go:323`，调用点 L294，confidence=0.90，exact-match）；`isMessageFormat`（`internal/agent/migrate.go:444`，调用点 L291，confidence=0.90，exact-match）；`transformAndCopyJsonl`（`internal/agent/migrate.go:482`，调用点 L305，confidence=0.90，exact-match）；`recordImportedTitle`（`internal/agent/migrate.go:613`，调用点 L317，confidence=0.90，exact-match）；`copyBranchMetaSidecar`（`internal/agent/migrate.go:782`，调用点 L312，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `MigrateLegacySessions`（`internal/agent/migrate.go:70`，调用点 L71，confidence=0.90，exact-match，待源码复核）；`MigrateLegacySessionsFromConfigDir`（`internal/agent/migrate.go:78`，调用点 L79，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/migrate.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `importJsonlSessions` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L274、L287、L297、L305、L313 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `importJsonlSessions` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/migrate.go|importJsonlSessions` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/migrate.go:271-321` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `importJsonlSessions` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G25 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/migrate.go|importJsonlSessions` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-034 · 圈复杂度 · `jsonlSessionDestDir`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-034 |
| Baseline key | `complexity\|internal/agent/migrate.go\|jsonlSessionDestDir` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/agent/migrate.go` |
| 代码行 | 323 |
| 函数/方法/包 | `jsonlSessionDestDir` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`jsonlSessionDestDir` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 323-347 行；可见控制流热点：L324 `if meta, ok, err := LoadBranchMeta(srcPath); err == nil && ok {`；L327 `if projectDir != nil && scope == "project" && meta.WorkspaceRoot != "" && dirExists(meta.WorkspaceR…`；L328 `if d := projectDir(meta.WorkspaceRoot); d != "" {`；L335 `if meta.Scope != "" {`；L341 `if projectDir != nil && meta.Workspace != "" && dirExists(meta.Workspace) {`；L342 `if d := projectDir(meta.Workspace); d != "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（LoadBranchMeta）、持久化/I-O（readLegacyMeta）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function jsonlSessionDestDir（internal/agent/migrate.go:323-347）
- **可信生产调用方（1）：** `importJsonlSessions`（`internal/agent/migrate.go:271`，调用点 L294，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `LoadBranchMeta`（`internal/agent/branch.go:91`，调用点 L324，confidence=0.90，exact-match）；`readLegacyMeta`（`internal/agent/migrate.go:572`，调用点 L339，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `dirExists`（`internal/agent/migrate.go:584`，调用点 L327/L341，confidence=0.70，exact-match，2 条边，待源码复核）；`BranchMeta::DefaultScope`（`internal/agent/branch.go:57`，调用点 L326，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/migrate.go`、`internal/agent/branch.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `jsonlSessionDestDir` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L324、L327、L335、L341、L342 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `jsonlSessionDestDir` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/migrate.go|jsonlSessionDestDir` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/migrate.go:323-347` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `jsonlSessionDestDir` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G25 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/migrate.go|jsonlSessionDestDir` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-038 · 圈复杂度 · `strandedSessionDestDir`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-038 |
| Baseline key | `complexity\|internal/agent/migrate.go\|strandedSessionDestDir` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/agent/migrate.go` |
| 代码行 | 756 |
| 函数/方法/包 | `strandedSessionDestDir` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`strandedSessionDestDir` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 756-776 行；可见控制流热点：L757 `if meta, ok, err := LoadBranchMeta(srcPath); err == nil && ok {`；L758 `if meta.DefaultScope() == "project" && meta.WorkspaceRoot != "" && dirExists(meta.WorkspaceRoot) {`；L759 `if d := projectDir(meta.WorkspaceRoot); d != "" {`；L765 `if meta.Scope != "" {`；L770 `if legacy.Workspace != "" && dirExists(legacy.Workspace) {`；L771 `if d := projectDir(legacy.Workspace); d != "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（LoadBranchMeta）、持久化/I-O（readLegacyMeta）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function strandedSessionDestDir（internal/agent/migrate.go:756-776）
- **可信生产调用方（1）：** `rehomeStrandedSessions`（`internal/agent/migrate.go:685`，调用点 L720，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `LoadBranchMeta`（`internal/agent/branch.go:91`，调用点 L757，confidence=0.90，exact-match）；`readLegacyMeta`（`internal/agent/migrate.go:572`，调用点 L769，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `dirExists`（`internal/agent/migrate.go:584`，调用点 L758/L770，confidence=0.70，exact-match，2 条边，待源码复核）；`BranchMeta::DefaultScope`（`internal/agent/branch.go:57`，调用点 L758，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/migrate.go`、`internal/agent/branch.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `strandedSessionDestDir` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L757、L758、L765、L770、L771 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `strandedSessionDestDir` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/migrate.go|strandedSessionDestDir` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/migrate.go:756-776` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `strandedSessionDestDir` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G25 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/migrate.go|strandedSessionDestDir` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-039 · 圈复杂度 · `copyFileIfExists`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-039 |
| Baseline key | `complexity\|internal/agent/migrate.go\|copyFileIfExists` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/agent/migrate.go` |
| 代码行 | 830 |
| 函数/方法/包 | `copyFileIfExists` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`copyFileIfExists` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 830-873 行；可见控制流热点：L832 `if err != nil {`；L838 `if info.IsDir() {`；L847 `if err != nil {`；L850 `if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {`；L858 `if _, err := tmp.Write(b); err != nil {`；L867 `if err := os.Rename(tmpPath, dst); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function copyFileIfExists（internal/agent/migrate.go:830-873）
- **可信生产调用方（1）：** `copySubagentArtifacts`（`internal/agent/migrate.go:810`，调用点 L822，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/migrate.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `copyFileIfExists` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L832、L838、L850、L858、L867 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `copyFileIfExists` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/migrate.go|copyFileIfExists` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/migrate.go:830-873` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `copyFileIfExists` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G25 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/migrate.go|copyFileIfExists` 下降或消失且无新增/恶化。
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
