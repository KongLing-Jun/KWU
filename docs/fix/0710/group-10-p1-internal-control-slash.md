# G10 · internal/control · slash 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G10 |
| 问题数量 | 10 |
| 整体优先级 | P1 |
| 涉及模块 | `internal/control` |
| 涉及文件 | `internal/control/slash.go`<br>`internal/control/auto_plan.go` |
| 主要治理目标 | 围绕 `internal/control/slash.go` 等治理对象降低复杂度/体量，统一处理 1 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | control 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 7 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 10 组；无同文件组依赖 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **59/100**，对象 `*Controller.managementNotice` 的分项为严重度 14.0、集中度 11.8、影响面 21.3、可执行性 12.0；最高超限 3.10x，文件内 7 条问题，CodeGraph 记录 0 个可信生产调用方、1 个待核候选。该对象同时命中 圈复杂度、函数有效行数，必须合并治理。

### 治理边界与组间关系

本组涉及 2 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-234` | `complexity\|internal/control/slash.go\|*Controller.managementNotice` | 圈复杂度 | `internal/control/slash.go` | `*Controller.managementNotice` | 420 | 31/10 | +21（3.10x） | P1 | 0 可信 + 1 待核 caller / 5 可信 + 10 待核 callee / 0 unresolved | 待修改 |
| `FN-033` | `func_lines\|internal/control/slash.go\|*Controller.managementNotice` | 函数有效行数 | `internal/control/slash.go` | `*Controller.managementNotice` | 420 | 94/80 | +14（1.18x） | P1 | 0 可信 + 1 待核 caller / 5 可信 + 10 待核 callee / 0 unresolved | 待修改 |
| `CX-206` | `complexity\|internal/control/auto_plan.go\|isLowRiskQuestion` | 圈复杂度 | `internal/control/auto_plan.go` | `isLowRiskQuestion` | 207 | 50/10 | +40（5.00x） | P2 | 2 可信 + 0 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-231` | `complexity\|internal/control/slash.go\|SlashArgItems` | 圈复杂度 | `internal/control/slash.go` | `SlashArgItems` | 57 | 16/10 | +6（1.60x） | P2 | 3 可信 + 0 待核 caller / 14 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-204` | `complexity\|internal/control/auto_plan.go\|autoPlanScore` | 圈复杂度 | `internal/control/auto_plan.go` | `autoPlanScore` | 96 | 15/10 | +5（1.50x） | P2 | 2 可信 + 0 待核 caller / 4 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-205` | `complexity\|internal/control/auto_plan.go\|isContextDependentShortReply` | 圈复杂度 | `internal/control/auto_plan.go` | `isContextDependentShortReply` | 139 | 11/10 | +1（1.10x） | P3 | 2 可信 + 0 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-232` | `complexity\|internal/control/slash.go\|mcpArgItems` | 圈复杂度 | `internal/control/slash.go` | `mcpArgItems` | 253 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-236` | `complexity\|internal/control/slash.go\|*Controller.memoryListText` | 圈复杂度 | `internal/control/slash.go` | `*Controller.memoryListText` | 739 | 14/10 | +4（1.40x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-233` | `complexity\|internal/control/slash.go\|skillArgItems` | 圈复杂度 | `internal/control/slash.go` | `skillArgItems` | 350 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-235` | `complexity\|internal/control/slash.go\|*Controller.expertNotice` | 圈复杂度 | `internal/control/slash.go` | `*Controller.expertNotice` | 517 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-234 · 圈复杂度 · `*Controller.managementNotice`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-234 |
| Baseline key | `complexity\|internal/control/slash.go\|*Controller.managementNotice` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 59） |
| 文件 | `internal/control/slash.go` |
| 代码行 | 420 |
| 函数/方法/包 | `*Controller.managementNotice` |
| Baseline / 当前复测 / 阈值 | 31 / 31 / 10 |
| 超出 | +21，3.10x |
| 同一治理对象关联问题 | `FN-033` |

#### 明确的问题描述

`*Controller.managementNotice` 的圈复杂度当前为 **31**，超过限制 **10**（+21，3.10x）。定义范围为 420-515 行；可见控制流热点：L422 `if len(fields) == 0 {`；L434 `case "/memory":`；L446 `if len(fields) >= 2 {`；L466 `if err := c.ReloadCommands(context.Background()); err != nil {`；L481 `if root == "" {`；L503 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（NewProfilePathResolver、ProfilePathResolver::HookTrustPath）、状态与控制（RunLegacyRescue、Controller::profileNotice、Controller::expertNotice、Controller::memoryV5Notice）、持久化/I-O（NewProfilePathResolver、ProfilePathResolver::HookTrustPath、Controller::profileNotice）、渲染/输出（Controller::profileNotice、Controller::expertNotice、Controller::memoryV5Notice）、网络/协议（Controller::providerListText、Controller::providerSwitchText）、权限/安全（ProfilePathResolver::HookTrustPath、Trust、TrustAt）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::managementNotice（internal/control/slash.go:420-515）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Controller::submitCommandOrTurn`（`internal/control/controller.go:698`，调用点 L809，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（5）：** `NewProfilePathResolver`（`internal/config/paths.go:81`，调用点 L490，confidence=0.90，import）；`ProfilePathResolver::HookTrustPath`（`internal/config/paths.go:239`，调用点 L490，confidence=0.90，exact-match）；`Trust`（`internal/hook/trust.go:49`，调用点 L488，confidence=0.90，import）；`TrustAt`（`internal/hook/trust.go:62`，调用点 L490，confidence=0.90，import）；`RunLegacyRescue`（`internal/migration/migration.go:78`，调用点 L443，confidence=0.90，import）
- **待源码复核被调候选（10）：** `Controller::profileNotice`（`internal/control/profile_management.go:74`，调用点 L439，confidence=0.70，instance-method，待源码复核）；`Controller::expertNotice`（`internal/control/slash.go:517`，调用点 L441，confidence=0.70，instance-method，待源码复核）；`Controller::memoryV5Notice`（`internal/control/slash.go:583`，调用点 L437，confidence=0.70，instance-method，待源码复核）；`Controller::modelListText`（`internal/control/slash.go:659`，调用点 L427，confidence=0.70，instance-method，待源码复核）；`Controller::providerListText`（`internal/control/slash.go:679`，调用点 L432，confidence=0.70，instance-method，待源码复核）；`Controller::providerSwitchText`（`internal/control/slash.go:709`，调用点 L430，confidence=0.70，instance-method，待源码复核）；`Controller::memoryListText`（`internal/control/slash.go:739`，调用点 L435，confidence=0.70，instance-method，待源码复核）；`Controller::skillListText`（`internal/control/slash.go:792`，调用点 L460，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Controller::submit`（`internal/control/controller.go:658`，调用点 L675，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/control`、`internal/config`、`internal/hook`、`internal/migration`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/controller_test.go`、`internal/control/slash_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/slash.go`、`internal/control/controller.go`、`internal/config/paths.go`、`internal/hook/trust.go`、`internal/control/controller_test.go`、`internal/control/slash_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.managementNotice` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L422、L438、L461、L477、L503 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Controller.managementNotice` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-234、FN-033 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/slash.go|*Controller.managementNotice` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/slash.go:420-515` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.managementNotice` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-234`、`FN-033` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/controller_test.go`、`internal/control/slash_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/slash.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/slash.go|*Controller.managementNotice` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-033 · 函数有效行数 · `*Controller.managementNotice`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-033 |
| Baseline key | `func_lines\|internal/control/slash.go\|*Controller.managementNotice` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P3，治理对象得分 59） |
| 文件 | `internal/control/slash.go` |
| 代码行 | 420 |
| 函数/方法/包 | `*Controller.managementNotice` |
| Baseline / 当前复测 / 阈值 | 94 / 94 / 80 |
| 超出 | +14，1.18x |
| 同一治理对象关联问题 | `CX-234` |

#### 明确的问题描述

`*Controller.managementNotice` 的有效代码行 **94**，超过限制 **80**（+14）。CodeGraph 定义物理范围为 420-515 行，并记录 15 个直接下游候选；候选职责分布为 配置/解析（NewProfilePathResolver、ProfilePathResolver::HookTrustPath）、状态与控制（RunLegacyRescue、Controller::profileNotice、Controller::expertNotice、Controller::memoryV5Notice）、持久化/I-O（NewProfilePathResolver、ProfilePathResolver::HookTrustPath、Controller::profileNotice）、渲染/输出（Controller::profileNotice、Controller::expertNotice、Controller::memoryV5Notice）、网络/协议（Controller::providerListText、Controller::providerSwitchText）、权限/安全（ProfilePathResolver::HookTrustPath、Trust、TrustAt）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::managementNotice（internal/control/slash.go:420-515）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Controller::submitCommandOrTurn`（`internal/control/controller.go:698`，调用点 L809，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（5）：** `NewProfilePathResolver`（`internal/config/paths.go:81`，调用点 L490，confidence=0.90，import）；`ProfilePathResolver::HookTrustPath`（`internal/config/paths.go:239`，调用点 L490，confidence=0.90，exact-match）；`Trust`（`internal/hook/trust.go:49`，调用点 L488，confidence=0.90，import）；`TrustAt`（`internal/hook/trust.go:62`，调用点 L490，confidence=0.90，import）；`RunLegacyRescue`（`internal/migration/migration.go:78`，调用点 L443，confidence=0.90，import）
- **待源码复核被调候选（10）：** `Controller::profileNotice`（`internal/control/profile_management.go:74`，调用点 L439，confidence=0.70，instance-method，待源码复核）；`Controller::expertNotice`（`internal/control/slash.go:517`，调用点 L441，confidence=0.70，instance-method，待源码复核）；`Controller::memoryV5Notice`（`internal/control/slash.go:583`，调用点 L437，confidence=0.70，instance-method，待源码复核）；`Controller::modelListText`（`internal/control/slash.go:659`，调用点 L427，confidence=0.70，instance-method，待源码复核）；`Controller::providerListText`（`internal/control/slash.go:679`，调用点 L432，confidence=0.70，instance-method，待源码复核）；`Controller::providerSwitchText`（`internal/control/slash.go:709`，调用点 L430，confidence=0.70，instance-method，待源码复核）；`Controller::memoryListText`（`internal/control/slash.go:739`，调用点 L435，confidence=0.70，instance-method，待源码复核）；`Controller::skillListText`（`internal/control/slash.go:792`，调用点 L460，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Controller::submit`（`internal/control/controller.go:658`，调用点 L675，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/control`、`internal/config`、`internal/hook`、`internal/migration`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/controller_test.go`、`internal/control/slash_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/slash.go`、`internal/control/controller.go`、`internal/config/paths.go`、`internal/hook/trust.go`、`internal/control/controller_test.go`、`internal/control/slash_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.managementNotice` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*Controller.managementNotice` 的 94 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-234、FN-033 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/control/slash.go|*Controller.managementNotice` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/slash.go:420-515` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.managementNotice` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-234`、`FN-033` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/controller_test.go`、`internal/control/slash_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/slash.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/control/slash.go|*Controller.managementNotice` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-206 · 圈复杂度 · `isLowRiskQuestion`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-206 |
| Baseline key | `complexity\|internal/control/auto_plan.go\|isLowRiskQuestion` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 57） |
| 文件 | `internal/control/auto_plan.go` |
| 代码行 | 207 |
| 函数/方法/包 | `isLowRiskQuestion` |
| Baseline / 当前复测 / 阈值 | 50 / 50 / 10 |
| 超出 | +40，5.00x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`isLowRiskQuestion` 的圈复杂度当前为 **50**，超过限制 **10**（+40，5.00x）。定义范围为 207-237 行；可见控制流热点：L210 `if strings.HasPrefix(lower, "what ") || strings.HasPrefix(normalized, "whats ") ||`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function isLowRiskQuestion（internal/control/auto_plan.go:207-237）
- **可信生产调用方（2）：** `TaskWarrantsPlanner`（`internal/control/auto_plan.go:81`，调用点 L93，confidence=0.90，exact-match）；`autoPlanScore`（`internal/control/auto_plan.go:96`，调用点 L109，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `containsAny`（`internal/control/auto_plan.go:256`，调用点 L234，confidence=0.70，exact-match，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/auto_plan.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `isLowRiskQuestion` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L210 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `isLowRiskQuestion` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/auto_plan.go|isLowRiskQuestion` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/auto_plan.go:207-237` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `isLowRiskQuestion` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/auto_plan.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/auto_plan.go|isLowRiskQuestion` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-231 · 圈复杂度 · `SlashArgItems`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-231 |
| Baseline key | `complexity\|internal/control/slash.go\|SlashArgItems` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 51） |
| 文件 | `internal/control/slash.go` |
| 代码行 | 57 |
| 函数/方法/包 | `SlashArgItems` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`SlashArgItems` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 57-99 行；可见控制流热点：L59 `if cmdEnd < 0 {`；L69 `case "/model":`；L75 `case "/hooks":`；L81 `case "/goal":`；L87 `case "/profile":`；L93 `case "/expert":`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（profileArgItems）、网络/协议（providerArgItems）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function SlashArgItems（internal/control/slash.go:57-99）
- **可信生产调用方（3）：** `App::SlashArgs`（`desktop/app.go:4494`，调用点 L4534，confidence=0.90，import）；`chatTUI::slashArgItems`（`internal/cli/complete.go:161`，调用点 L175，confidence=0.90，import）；`chatTUI::explicitSubcommandItems`（`internal/cli/complete.go:232`，调用点 L242，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（14）：** `expertArgItems`（`internal/control/slash.go:101`，调用点 L94，confidence=0.90，exact-match）；`goalArgItems`（`internal/control/slash.go:123`，调用点 L82，confidence=0.90，exact-match）；`autoPlanArgItems`（`internal/control/slash.go:135`，调用点 L80，confidence=0.90，exact-match）；`reasoningLanguageArgItems`（`internal/control/slash.go:145`，调用点 L84，confidence=0.90，exact-match）；`memoryV5ArgItems`（`internal/control/slash.go:156`，调用点 L86，confidence=0.90，exact-match）；`profileArgItems`（`internal/control/slash.go:167`，调用点 L88，confidence=0.90，exact-match）；`languageArgItems`（`internal/control/slash.go:188`，调用点 L92，confidence=0.90，exact-match）；`effortArgItems`（`internal/control/slash.go:227`，调用点 L78，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `themeArgItems`（`internal/control/slash.go:199`，调用点 L90，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `App::SlashArgs`（`desktop/app.go:4494`，调用点 L4534，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/control`、`desktop`、`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/slash_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/slash.go`、`desktop/app.go`、`internal/cli/complete.go`、`internal/control/slash_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`

#### 待修改项

- [ ] 先冻结 `SlashArgItems` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L59、L71、L79、L85、L93 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `SlashArgItems` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/slash.go|SlashArgItems` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/slash.go:57-99` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `SlashArgItems` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/slash_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/slash.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/slash.go|SlashArgItems` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-204 · 圈复杂度 · `autoPlanScore`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-204 |
| Baseline key | `complexity\|internal/control/auto_plan.go\|autoPlanScore` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 49） |
| 文件 | `internal/control/auto_plan.go` |
| 代码行 | 96 |
| 函数/方法/包 | `autoPlanScore` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`autoPlanScore` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 96-137 行；可见控制流热点：L99 `if text == "" || strings.HasPrefix(text, "/") || strings.HasPrefix(text, PlanModeMarker) {`；L105 `if isContextDependentShortReply(text) {`；L114 `if utf8.RuneCountInString(text) >= 160 {`；L120 `if strings.Count(text, "\n") >= 2 {`；L126 `if containsAny(lower, multiSurfaceTerms) {`；L132 `if strings.Count(text, "@") >= 2 || strings.Count(lower, ".go")+`。CodeGraph 记录的下游职责候选主要涉及 任务/并发（stripActiveGoalBlock）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function autoPlanScore（internal/control/auto_plan.go:96-137）
- **可信生产调用方（2）：** `Controller::shouldAutoPlan`（`internal/control/auto_plan.go:47`，调用点 L56，confidence=0.90，exact-match）；`NewPlannerGate`（`internal/control/auto_plan.go:289`，调用点 L297，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `isContextDependentShortReply`（`internal/control/auto_plan.go:139`，调用点 L105，confidence=0.90，exact-match）；`isLowRiskQuestion`（`internal/control/auto_plan.go:207`，调用点 L109，confidence=0.90，exact-match）；`stripActiveGoalBlock`（`internal/control/auto_plan.go:239`，调用点 L98，confidence=0.90，exact-match）；`IsSyntheticUserMessage`（`internal/control/input.go:126`，调用点 L102，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `containsAny`（`internal/control/auto_plan.go:256`，调用点 L123/L126/L129，confidence=0.70，exact-match，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** `Build`（`internal/boot/boot.go:152`，调用点 L1105，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/auto_plan_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/auto_plan.go`、`internal/control/input.go`、`internal/control/auto_plan_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`

#### 待修改项

- [ ] 先冻结 `autoPlanScore` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L99、L109、L117、L126、L132 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `autoPlanScore` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/auto_plan.go|autoPlanScore` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/auto_plan.go:96-137` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `autoPlanScore` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/auto_plan_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/auto_plan.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/auto_plan.go|autoPlanScore` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-205 · 圈复杂度 · `isContextDependentShortReply`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-205 |
| Baseline key | `complexity\|internal/control/auto_plan.go\|isContextDependentShortReply` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 43） |
| 文件 | `internal/control/auto_plan.go` |
| 代码行 | 139 |
| 函数/方法/包 | `isContextDependentShortReply` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`isContextDependentShortReply` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 139-163 行；可见控制流热点：L141 `if text == "" || strings.ContainsAny(text, "\n\r") {`；L144 `if directOptionReplyRE.MatchString(text) || prefixedOptionReplyRE.MatchString(text) {`；L148 `if containsAny(lower, complexIntentTerms) || containsAny(lower, lowRiskWorkRequestTerms) {`；L154 `if utf8.RuneCountInString(text) > 16 {`；L157 `for _, prefix := range shortContextReplyPrefixes {`；L158 `if strings.HasPrefix(lower, prefix) {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function isContextDependentShortReply（internal/control/auto_plan.go:139-163）
- **可信生产调用方（2）：** `TaskWarrantsPlanner`（`internal/control/auto_plan.go:81`，调用点 L90，confidence=0.90，exact-match）；`autoPlanScore`（`internal/control/auto_plan.go:96`，调用点 L105，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `containsAny`（`internal/control/auto_plan.go:256`，调用点 L148，confidence=0.70，exact-match，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/auto_plan.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `isContextDependentShortReply` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L141、L148、L151、L157、L158 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `isContextDependentShortReply` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/auto_plan.go|isContextDependentShortReply` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/auto_plan.go:139-163` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `isContextDependentShortReply` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/auto_plan.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/auto_plan.go|isContextDependentShortReply` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-232 · 圈复杂度 · `mcpArgItems`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-232 |
| Baseline key | `complexity\|internal/control/slash.go\|mcpArgItems` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/control/slash.go` |
| 代码行 | 253 |
| 函数/方法/包 | `mcpArgItems` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`mcpArgItems` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 253-303 行；可见控制流热点：L254 `if len(prior) <= 1 {`；L265 `case "remove", "rm":`；L274 `case "show", "tools":`；L279 `for _, name := range allMCPArgNames(d) {`；L288 `for _, name := range d.DisconnectedMCP {`；L293 `if strings.HasPrefix(cur, "-") {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function mcpArgItems（internal/control/slash.go:253-303）
- **可信生产调用方（1）：** `SlashArgItems`（`internal/control/slash.go:57`，调用点 L68，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `allMCPArgNames`（`internal/control/slash.go:305`，调用点 L279，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::SlashArgs`（`desktop/app.go:4494`，调用点 L4534，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/slash.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `mcpArgItems` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L254、L266、L275、L284、L293 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `mcpArgItems` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/slash.go|mcpArgItems` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/slash.go:253-303` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `mcpArgItems` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/slash.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/slash.go|mcpArgItems` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-236 · 圈复杂度 · `*Controller.memoryListText`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-236 |
| Baseline key | `complexity\|internal/control/slash.go\|*Controller.memoryListText` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/control/slash.go` |
| 代码行 | 739 |
| 函数/方法/包 | `*Controller.memoryListText` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Controller.memoryListText` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 739-779 行；可见控制流热点：L741 `if mem == nil {`；L750 `if len(mem.Docs) > 0 {`；L756 `if len(saved) > 0 {`；L761 `for _, m := range saved {`；L766 `if b.Len() > 0 {`；L772 `if !m.ArchivedAt.IsZero() {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::memoryListText（internal/control/slash.go:739-779）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Controller::managementNotice`（`internal/control/slash.go:420`，调用点 L435，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `memoryDisplayTitle`（`internal/control/slash.go:781`，调用点 L762/L775，confidence=0.90，exact-match，2 条边）；`memoryOneLine`（`internal/control/slash.go:788`，调用点 L762/L775，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（2）：** `Registry::Len`（`internal/tool/tool.go:242`，调用点 L757/L766，confidence=0.70，instance-method，2 条边，待源码复核）；`memoryManager::current`（`internal/control/memory.go:49`，调用点 L740，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/control`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/slash_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/slash.go`、`internal/tool/tool.go`、`internal/control/memory.go`、`internal/control/slash_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.memoryListText` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L741、L752、L757、L766、L772 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `*Controller.memoryListText` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/slash.go|*Controller.memoryListText` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/slash.go:739-779` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.memoryListText` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/slash_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/slash.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/slash.go|*Controller.memoryListText` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-233 · 圈复杂度 · `skillArgItems`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-233 |
| Baseline key | `complexity\|internal/control/slash.go\|skillArgItems` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/control/slash.go` |
| 代码行 | 350 |
| 函数/方法/包 | `skillArgItems` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`skillArgItems` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 350-382 行；可见控制流热点：L351 `if len(prior) <= 1 {`；L360 `if (prior[1] == "show" || prior[1] == "cat") && len(prior) == 2 {`；L362 `for _, s := range d.Skills {`；L369 `for _, s := range d.Skills {`；L374 `if prior[1] == "enable" && len(prior) == 2 {`；L376 `for _, s := range d.DisabledSkills {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function skillArgItems（internal/control/slash.go:350-382）
- **可信生产调用方（1）：** `SlashArgItems`（`internal/control/slash.go:57`，调用点 L74，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::SlashArgs`（`desktop/app.go:4494`，调用点 L4534，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/slash.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `skillArgItems` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L351、L362、L367、L374、L376 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `skillArgItems` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/slash.go|skillArgItems` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/slash.go:350-382` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `skillArgItems` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/slash.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/slash.go|skillArgItems` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-235 · 圈复杂度 · `*Controller.expertNotice`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-235 |
| Baseline key | `complexity\|internal/control/slash.go\|*Controller.expertNotice` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/control/slash.go` |
| 代码行 | 517 |
| 函数/方法/包 | `*Controller.expertNotice` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Controller.expertNotice` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 517-563 行；可见控制流热点：L518 `if len(fields) == 1 {`；L527 `switch strings.ToLower(fields[1]) {`；L530 `case "use":`；L537 `if errors.Is(err, ErrExpertSwitchRequiresNewSession) {`；L547 `if err != nil {`；L555 `if state.SelectedID == "" {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（Controller::expertListText）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::expertNotice（internal/control/slash.go:517-563）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Controller::managementNotice`（`internal/control/slash.go:420`，调用点 L441，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `Controller::expertListText`（`internal/control/slash.go:565`，调用点 L529，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/slash.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.expertNotice` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L518、L528、L536、L545、L555 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Controller.expertNotice` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/slash.go|*Controller.expertNotice` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/slash.go:517-563` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.expertNotice` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/slash.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/slash.go|*Controller.expertNotice` 下降或消失且无新增/恶化。
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
