# G18 · internal/migration · refs 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G18 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/migration`、`internal/control` |
| 涉及文件 | `internal/migration/migration.go`<br>`internal/control/goal.go`<br>`internal/control/refs.go` |
| 主要治理目标 | 围绕 `internal/control/refs.go` 等治理对象降低复杂度/体量，统一处理 1 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | migration / control 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 6 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 18 组；先于 G31 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **50/100**，对象 `migrateLegacySessionSources` 的分项为严重度 9.5、集中度 10.3、影响面 10.4、可执行性 20.0；最高超限 2.00x，文件内 4 条问题，CodeGraph 记录 2 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数，必须合并治理。

### 治理边界与组间关系

本组涉及 3 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G31，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-318` | `complexity\|internal/migration/migration.go\|migrateLegacySessionSources` | 圈复杂度 | `internal/migration/migration.go` | `migrateLegacySessionSources` | 294 | 20/10 | +10（2.00x） | P2 | 2 可信 + 0 待核 caller / 3 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `FN-044` | `func_lines\|internal/migration/migration.go\|migrateLegacySessionSources` | 函数有效行数 | `internal/migration/migration.go` | `migrateLegacySessionSources` | 294 | 90/80 | +10（1.13x） | P2 | 2 可信 + 0 待核 caller / 3 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-221` | `complexity\|internal/control/goal.go\|*goalMachine.advance` | 圈复杂度 | `internal/control/goal.go` | `*goalMachine.advance` | 186 | 20/10 | +10（2.00x） | P2 | 1 可信 + 0 待核 caller / 3 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-229` | `complexity\|internal/control/refs.go\|readFileRefUnscoped` | 圈复杂度 | `internal/control/refs.go` | `readFileRefUnscoped` | 550 | 20/10 | +10（2.00x） | P2 | 1 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-228` | `complexity\|internal/control/refs.go\|readFileRef` | 圈复杂度 | `internal/control/refs.go` | `readFileRef` | 481 | 17/10 | +7（1.70x） | P2 | 1 可信 + 0 待核 caller / 7 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-225` | `complexity\|internal/control/refs.go\|*Controller.detectRefsMode` | 圈复杂度 | `internal/control/refs.go` | `*Controller.detectRefsMode` | 112 | 12/10 | +2（1.20x） | P3 | 0 可信 + 2 待核 caller / 5 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-226` | `complexity\|internal/control/refs.go\|visionFileImageDataURL` | 圈复杂度 | `internal/control/refs.go` | `visionFileImageDataURL` | 177 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-227` | `complexity\|internal/control/refs.go\|resolveBareNames` | 圈复杂度 | `internal/control/refs.go` | `resolveBareNames` | 242 | 14/10 | +4（1.40x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-317` | `complexity\|internal/migration/migration.go\|migrateLegacyMemorySources` | 圈复杂度 | `internal/migration/migration.go` | `migrateLegacyMemorySources` | 132 | 15/10 | +5（1.50x） | P3 | 2 可信 + 0 待核 caller / 4 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-316` | `complexity\|internal/migration/migration.go\|Result.Summary` | 圈复杂度 | `internal/migration/migration.go` | `Result.Summary` | 41 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-318 · 圈复杂度 · `migrateLegacySessionSources`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-318 |
| Baseline key | `complexity\|internal/migration/migration.go\|migrateLegacySessionSources` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 50） |
| 文件 | `internal/migration/migration.go` |
| 代码行 | 294 |
| 函数/方法/包 | `migrateLegacySessionSources` |
| Baseline / 当前复测 / 阈值 | 20 / 20 / 10 |
| 超出 | +10，2.00x |
| 同一治理对象关联问题 | `FN-044` |

#### 明确的问题描述

`migrateLegacySessionSources` 的圈复杂度当前为 **20**，超过限制 **10**（+10，2.00x）。定义范围为 294-389 行；可见控制流热点：L296 `if strings.TrimSpace(dest) == "" {`；L324 `if err != nil {`；L342 `if home, herr := os.UserHomeDir(); herr == nil {`；L360 `if strings.TrimSpace(src.dir) == "" {`；L373 `if err != nil {`；L385 `if verbose && len(result.imports) == 0 && len(result.errs) == 0 {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（LegacyUserConfigPaths）、渲染/输出（Sink::Emit）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function migrateLegacySessionSources（internal/migration/migration.go:294-389）
- **可信生产调用方（2）：** `RunLegacyRescue`（`internal/migration/migration.go:78`，调用点 L100，confidence=0.90，exact-match）；`MigrateLegacySessionSources`（`internal/migration/migration.go:117`，调用点 L119，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `MemoryUserDir`（`internal/config/paths.go:491`，调用点 L316/L319/L333，confidence=0.90，import，3 条边）；`LegacyUserConfigPaths`（`internal/config/paths.go:388`，调用点 L347，confidence=0.90，import）；`SessionDir`（`internal/config/paths.go:434`，调用点 L295，confidence=0.90，import）
- **待源码复核被调候选（2）：** `Sink::Emit`（`internal/event/event.go:362`，调用点 L376/L382/L386，confidence=0.80，instance-method，3 条边，待源码复核）；`samePath`（`internal/migration/migration.go:391`，调用点 L319，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `Build`（`internal/boot/boot.go:152`，调用点 L225，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/migration`、`internal/config`、`internal/event`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/migration/migration_test.go`
- **需同步检查：** `internal/migration/migration.go`、`internal/config/paths.go`、`internal/event/event.go`、`internal/migration/migration_test.go`

#### 待修改项

- [ ] 逐段核对 L296、L327、L359、L368、L385 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `migrateLegacySessionSources` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-318、FN-044 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/migration/migration.go|migrateLegacySessionSources` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/migration/migration.go:294-389` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `migrateLegacySessionSources` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-318`、`FN-044` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/migration/migration_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/migration`。
- [ ] 运行相关包 vet：`go vet ./internal/migration`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/migration/migration.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/migration/migration.go|migrateLegacySessionSources` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-044 · 函数有效行数 · `migrateLegacySessionSources`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-044 |
| Baseline key | `func_lines\|internal/migration/migration.go\|migrateLegacySessionSources` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 50） |
| 文件 | `internal/migration/migration.go` |
| 代码行 | 294 |
| 函数/方法/包 | `migrateLegacySessionSources` |
| Baseline / 当前复测 / 阈值 | 90 / 90 / 80 |
| 超出 | +10，1.13x |
| 同一治理对象关联问题 | `CX-318` |

#### 明确的问题描述

`migrateLegacySessionSources` 的有效代码行 **90**，超过限制 **80**（+10）。CodeGraph 定义物理范围为 294-389 行，并记录 5 个直接下游候选；候选职责分布为 配置/解析（LegacyUserConfigPaths）、渲染/输出（Sink::Emit）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function migrateLegacySessionSources（internal/migration/migration.go:294-389）
- **可信生产调用方（2）：** `RunLegacyRescue`（`internal/migration/migration.go:78`，调用点 L100，confidence=0.90，exact-match）；`MigrateLegacySessionSources`（`internal/migration/migration.go:117`，调用点 L119，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `MemoryUserDir`（`internal/config/paths.go:491`，调用点 L316/L319/L333，confidence=0.90，import，3 条边）；`LegacyUserConfigPaths`（`internal/config/paths.go:388`，调用点 L347，confidence=0.90，import）；`SessionDir`（`internal/config/paths.go:434`，调用点 L295，confidence=0.90，import）
- **待源码复核被调候选（2）：** `Sink::Emit`（`internal/event/event.go:362`，调用点 L376/L382/L386，confidence=0.80，instance-method，3 条边，待源码复核）；`samePath`（`internal/migration/migration.go:391`，调用点 L319，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `Build`（`internal/boot/boot.go:152`，调用点 L225，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/migration`、`internal/config`、`internal/event`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/migration/migration_test.go`
- **需同步检查：** `internal/migration/migration.go`、`internal/config/paths.go`、`internal/event/event.go`、`internal/migration/migration_test.go`

#### 待修改项

- [ ] 把 `migrateLegacySessionSources` 的 90 个有效行按 配置/解析、渲染/输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L352 `Back-fill v0.x sessions from the current user config session directory as`、L353 `well. This covers users whose platform config root was redirected before the`、L354 `Go rewrite; their event logs can already live where v2 stores sessions.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-318、FN-044 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/migration/migration.go|migrateLegacySessionSources` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/migration/migration.go:294-389` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `migrateLegacySessionSources` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-318`、`FN-044` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/migration/migration_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/migration`。
- [ ] 运行相关包 vet：`go vet ./internal/migration`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/migration/migration.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/migration/migration.go|migrateLegacySessionSources` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-221 · 圈复杂度 · `*goalMachine.advance`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-221 |
| Baseline key | `complexity\|internal/control/goal.go\|*goalMachine.advance` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 49） |
| 文件 | `internal/control/goal.go` |
| 代码行 | 186 |
| 函数/方法/包 | `*goalMachine.advance` |
| Baseline / 当前复测 / 阈值 | 20 / 20 / 10 |
| 超出 | +10，2.00x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*goalMachine.advance` 的圈复杂度当前为 **20**，超过限制 **10**（+10，2.00x）。定义范围为 186-270 行；可见控制流热点：L189 `if strings.TrimSpace(g.goal) == "" || g.status != GoalStatusRunning {`；L196 `if incomplete := formatIncompleteTodos(in.todos, in.readiness); len(incomplete) > 0 && (g.strict ||…`；L220 `case GoalStatusBlocked:`；L231 `if g.blocks >= 3 {`；L246 `if in.toolCalled {`；L266 `if notice != "" {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（goalMachine::buildStateLocked）、渲染/输出（formatIncompleteTodos）、任务/并发（sameGoalBlock、cleanGoalBlockReason、goalMachine::buildStateLocked）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method goalMachine::advance（internal/control/goal.go:186-270）
- **可信生产调用方（1）：** `turnOrchestrator::advanceGoalAfterTurn`（`internal/control/turn_orchestrator.go:149`，调用点 L159，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `formatIncompleteTodos`（`internal/control/goal.go:320`，调用点 L196，confidence=0.90，exact-match）；`sameGoalBlock`（`internal/control/goal.go:372`，调用点 L225，confidence=0.90，exact-match）；`cleanGoalBlockReason`（`internal/control/goal.go:376`，调用点 L221，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `goalMachine::buildStateLocked`（`internal/control/goal.go:276`，调用点 L267，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/goal.go`、`internal/control/turn_orchestrator.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*goalMachine.advance` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L189、L196、L225、L246、L266 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、渲染/输出、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*goalMachine.advance` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/goal.go|*goalMachine.advance` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/goal.go:186-270` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*goalMachine.advance` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/goal.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/goal.go|*goalMachine.advance` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-229 · 圈复杂度 · `readFileRefUnscoped`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-229 |
| Baseline key | `complexity\|internal/control/refs.go\|readFileRefUnscoped` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 49） |
| 文件 | `internal/control/refs.go` |
| 代码行 | 550 |
| 函数/方法/包 | `readFileRefUnscoped` |
| Baseline / 当前复测 / 阈值 | 20 / 20 / 10 |
| 超出 | +10，2.00x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`readFileRefUnscoped` 的圈复杂度当前为 **20**，超过限制 **10**（+10，2.00x）。定义范围为 550-623 行；可见控制流热点：L552 `if err != nil {`；L562 `if n >= maxDirEntries {`；L570 `case ".git", "node_modules", ".DS_Store", "__pycache__", ".idea", ".vscode":`；L587 `if n >= maxDirEntries {`；L608 `if rerr != nil && rerr != io.ErrUnexpectedEOF && rerr != io.EOF {`；L619 `if n > maxFileRefBytes {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（imageFileRefNote、readPDFRef）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function readFileRefUnscoped（internal/control/refs.go:550-623）
- **可信生产调用方（1）：** `readFileRef`（`internal/control/refs.go:481`，调用点 L487，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `imageFileRefNote`（`internal/control/refs.go:625`，调用点 L614，confidence=0.90，exact-match）；`readPDFRef`（`internal/control/refs.go:703`，调用点 L597，confidence=0.90，exact-match）；`imageMime`（`internal/control/refs.go:842`，调用点 L613，confidence=0.90，exact-match）；`min`（`internal/profile/selector.go:451`，调用点 L616，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::resolveRefs`（`internal/control/refs.go:432`，调用点 L446，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`、`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/refs.go`、`internal/profile/selector.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `readFileRefUnscoped` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L552、L565、L579、L601、L619 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `readFileRefUnscoped` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/refs.go|readFileRefUnscoped` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/refs.go:550-623` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `readFileRefUnscoped` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/refs.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/refs.go|readFileRefUnscoped` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-228 · 圈复杂度 · `readFileRef`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-228 |
| Baseline key | `complexity\|internal/control/refs.go\|readFileRef` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 48） |
| 文件 | `internal/control/refs.go` |
| 代码行 | 481 |
| 函数/方法/包 | `readFileRef` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`readFileRef` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 481-546 行；可见控制流热点：L483 `if !ok {`；L497 `if rerr != nil {`；L506 `if info.IsDir() {`；L519 `if strings.EqualFold(filepath.Ext(rel), ".pdf") {`；L531 `if rerr != nil && rerr != io.ErrUnexpectedEOF && rerr != io.EOF {`；L542 `if n > maxFileRefBytes {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（resolveAbsRef）、持久化/I-O（readFileRefUnscoped、imageFileRefNote、readPDFRef）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function readFileRef（internal/control/refs.go:481-546）
- **可信生产调用方（1）：** `Controller::resolveRefs`（`internal/control/refs.go:432`，调用点 L446，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（7）：** `readFileRefUnscoped`（`internal/control/refs.go:550`，调用点 L487，confidence=0.90，exact-match）；`imageFileRefNote`（`internal/control/refs.go:625`，调用点 L537，confidence=0.90，exact-match）；`walkRootDir`（`internal/control/refs.go:635`，调用点 L509，confidence=0.90，exact-match）；`resolveAbsRef`（`internal/control/refs.go:680`，调用点 L482，confidence=0.90，exact-match）；`readPDFRef`（`internal/control/refs.go:703`，调用点 L520，confidence=0.90，exact-match）；`imageMime`（`internal/control/refs.go:842`，调用点 L536，confidence=0.90，exact-match）；`min`（`internal/profile/selector.go:451`，调用点 L539，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::resolveRefs`（`internal/control/refs.go:432`，调用点 L446，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`、`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/refs_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/refs.go`、`internal/control/refs_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`

#### 待修改项

- [ ] 先冻结 `readFileRef` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L483、L497、L513、L531、L542 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `readFileRef` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/refs.go|readFileRef` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/refs.go:481-546` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `readFileRef` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/refs_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/refs.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/refs.go|readFileRef` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-225 · 圈复杂度 · `*Controller.detectRefsMode`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-225 |
| Baseline key | `complexity\|internal/control/refs.go\|*Controller.detectRefsMode` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/control/refs.go` |
| 代码行 | 112 |
| 函数/方法/包 | `*Controller.detectRefsMode` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Controller.detectRefsMode` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 112-145 行；可见控制流热点：L114 `for _, n := range c.mcp.serverNames() {`；L119 `for _, tok := range parseRefTokens(line) {`；L124 `if c.workspaceRoot != "" {`；L125 `if rel, ok := workspaceRefPath(tok, c.workspaceRoot); ok {`；L134 `if scopedOnly {`；L137 `if r, ok := classifyRef(tok, known, func(p string) bool {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseRefTokens）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::detectRefsMode（internal/control/refs.go:112-145）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `Controller::detectRefs`（`internal/control/refs.go:108`，调用点 L109，confidence=0.70，instance-method，待源码复核）；`Controller::resolveRefs`（`internal/control/refs.go:432`，调用点 L433，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（5）：** `mcpManager::serverNames`（`internal/control/mcp.go:131`，调用点 L114，confidence=0.90，exact-match）；`parseRefTokens`（`internal/control/refs.go:60`，调用点 L119，confidence=0.90，exact-match）；`classifyRef`（`internal/control/refs.go:78`，调用点 L137，confidence=0.90，exact-match）；`isAttachmentRef`（`internal/control/refs.go:94`，调用点 L127，confidence=0.90，exact-match）；`workspaceRefPath`（`internal/control/refs.go:390`，调用点 L125，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `isImageAttachmentRef`（`internal/control/refs.go:98`，调用点 L127，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `Controller::HasRefs`（`internal/control/refs.go:149`，调用点 L150，confidence=0.70，instance-method，待源码复核）；`Controller::resolveRefs`（`internal/control/refs.go:432`，调用点 L433，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/refs.go`、`internal/control/mcp.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.detectRefsMode` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L114、L120、L125、L127、L137 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Controller.detectRefsMode` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/refs.go|*Controller.detectRefsMode` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/refs.go:112-145` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.detectRefsMode` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/refs.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/refs.go|*Controller.detectRefsMode` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-226 · 圈复杂度 · `visionFileImageDataURL`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-226 |
| Baseline key | `complexity\|internal/control/refs.go\|visionFileImageDataURL` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/control/refs.go` |
| 代码行 | 177 |
| 函数/方法/包 | `visionFileImageDataURL` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`visionFileImageDataURL` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 177-220 行；可见控制流热点：L179 `if !ok {`；L187 `if err != nil {`；L198 `if err != nil {`；L201 `if info.Mode()&os.ModeSymlink != 0 {`；L208 `if err != nil {`；L216 `if !os.SameFile(info, opened) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（resolveAbsRef）、持久化/I-O（dataURLFromImageReader）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function visionFileImageDataURL（internal/control/refs.go:177-220）
- **可信生产调用方（1）：** `visionRefImageDataURL`（`internal/control/refs.go:166`，调用点 L171，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `dataURLFromImageReader`（`internal/control/refs.go:222`，调用点 L219，confidence=0.90，exact-match）；`resolveAbsRef`（`internal/control/refs.go:680`，调用点 L178，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/refs.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `visionFileImageDataURL` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L179、L187、L201、L208、L216 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `visionFileImageDataURL` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/refs.go|visionFileImageDataURL` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/refs.go:177-220` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `visionFileImageDataURL` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/refs.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/refs.go|visionFileImageDataURL` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-227 · 圈复杂度 · `resolveBareNames`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-227 |
| Baseline key | `complexity\|internal/control/refs.go\|resolveBareNames` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/control/refs.go` |
| 代码行 | 242 |
| 函数/方法/包 | `resolveBareNames` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`resolveBareNames` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 242-287 行；可见控制流热点：L245 `for i := range refs {`；L250 `if workspaceRoot != "" {`；L259 `if len(names) == 0 {`；L268 `if wErr != nil || found == len(names) {`；L272 `switch d.Name() {`；L278 `if r, ok := need[d.Name()]; ok {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function resolveBareNames（internal/control/refs.go:242-287）
- **可信生产调用方（1）：** `Controller::resolveRefs`（`internal/control/refs.go:432`，调用点 L434，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `isSafeBareRefName`（`internal/control/refs.go:289`，调用点 L247，confidence=0.90，exact-match）；`workspaceRefPath`（`internal/control/refs.go:390`，调用点 L251，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::resolveRefs`（`internal/control/refs.go:432`，调用点 L434，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/refs_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/refs.go`、`internal/control/refs_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`

#### 待修改项

- [ ] 先冻结 `resolveBareNames` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L245、L251、L264、L272、L278 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `resolveBareNames` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/refs.go|resolveBareNames` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/refs.go:242-287` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `resolveBareNames` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/refs_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/refs.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/refs.go|resolveBareNames` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-317 · 圈复杂度 · `migrateLegacyMemorySources`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-317 |
| Baseline key | `complexity\|internal/migration/migration.go\|migrateLegacyMemorySources` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 42） |
| 文件 | `internal/migration/migration.go` |
| 代码行 | 132 |
| 函数/方法/包 | `migrateLegacyMemorySources` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`migrateLegacyMemorySources` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 132-181 行；可见控制流热点：L134 `if strings.TrimSpace(dest) == "" {`；L149 `if home, herr := os.UserHomeDir(); herr == nil {`；L158 `for _, src := range sources {`；L160 `if key == "" || seen[key] {`；L167 `if verbose {`；L177 `if verbose && len(result.imports) == 0 && len(result.errs) == 0 {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（LegacyUserConfigPaths）、渲染/输出（Sink::Emit）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function migrateLegacyMemorySources（internal/migration/migration.go:132-181）
- **可信生产调用方（2）：** `RunLegacyRescue`（`internal/migration/migration.go:78`，调用点 L96，confidence=0.90，exact-match）；`MigrateLegacyMemorySources`（`internal/migration/migration.go:109`，调用点 L111，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `LegacyUserConfigPaths`（`internal/config/paths.go:388`，调用点 L152，confidence=0.90，import）；`MemoryUserDir`（`internal/config/paths.go:491`，调用点 L133，confidence=0.90，import）；`copyLegacyMemoryRoot`（`internal/migration/migration.go:183`，调用点 L164，confidence=0.90，exact-match）；`cleanAbs`（`internal/migration/migration.go:397`，调用点 L159，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Sink::Emit`（`internal/event/event.go:362`，调用点 L168/L174/L178，confidence=0.80，instance-method，3 条边，待源码复核）；`samePath`（`internal/migration/migration.go:391`，调用点 L144，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `Build`（`internal/boot/boot.go:152`，调用点 L224，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/migration`、`internal/config`、`internal/event`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/migration/migration_test.go`
- **需同步检查：** `internal/migration/migration.go`、`internal/config/paths.go`、`internal/migration/migration_test.go`

#### 待修改项

- [ ] 逐段核对 L134、L149、L160、L167、L177 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `migrateLegacyMemorySources` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/migration/migration.go|migrateLegacyMemorySources` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/migration/migration.go:132-181` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `migrateLegacyMemorySources` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/migration/migration_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/migration`。
- [ ] 运行相关包 vet：`go vet ./internal/migration`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/migration/migration.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/migration/migration.go|migrateLegacyMemorySources` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-316 · 圈复杂度 · `Result.Summary`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-316 |
| Baseline key | `complexity\|internal/migration/migration.go\|Result.Summary` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 32） |
| 文件 | `internal/migration/migration.go` |
| 代码行 | 41 |
| 函数/方法/包 | `Result.Summary` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Result.Summary` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 41-74 行；可见控制流热点：L43 `for _, imp := range r.SessionImports {`；L51 `if r.ConfigErr != nil {`；L56 `switch {`；L59 `case r.Config != nil || importedMemory > 0 || importedSessions > 0:`；L61 `if r.Config != nil {`；L67 `if importedSessions > 0 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Result::Summary（internal/migration/migration.go:41-74）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `RunLegacyRescue`（`internal/migration/migration.go:78`，调用点 L103，confidence=0.80，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/migration`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/migration/migration_test.go`
- **需同步检查：** `internal/migration/migration.go`、`internal/migration/migration_test.go`

#### 待修改项

- [ ] 逐段核对 L43、L51、L57、L61、L67 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `Result.Summary` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/migration/migration.go|Result.Summary` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/migration/migration.go:41-74` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Result.Summary` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/migration/migration_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/migration`。
- [ ] 运行相关包 vet：`go vet ./internal/migration`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/migration/migration.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/migration/migration.go|Result.Summary` 下降或消失且无新增/恶化。
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
