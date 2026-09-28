# G06 · Controller 会话控制与提交链路

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G06 |
| 问题数量 | 10 |
| 整体优先级 | P1 |
| 涉及模块 | `internal/control` |
| 涉及文件 | `internal/control/controller.go` |
| 主要治理目标 | 围绕 `internal/control/controller.go` 等治理对象降低复杂度/体量，统一处理 1 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | control 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614`、`origin/expert-202607081622`；单文件最多集中 14 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 6 组；先于 G31 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 并发、取消或事件顺序；协议、序列化或流式事件兼容；会话、状态或恢复语义；持久化、迁移或文件格式兼容；权限、安全或凭据边界；配置、平台或向后兼容；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **61/100**，对象 `*Controller.submitCommandOrTurn` 的分项为严重度 16.6、集中度 18.9、影响面 22.0、可执行性 4.0；最高超限 3.50x，文件内 14 条问题，CodeGraph 记录 0 个可信生产调用方、2 个待核候选。该对象同时命中 圈复杂度、函数有效行数、嵌套深度，必须合并治理。

### 治理边界与组间关系

本组问题集中在同一文件，建议由单一负责人统一处理。没有同文件前置组。后续同文件组：G31，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-209` | `complexity\|internal/control/controller.go\|*Controller.submitCommandOrTurn` | 圈复杂度 | `internal/control/controller.go` | `*Controller.submitCommandOrTurn` | 698 | 35/10 | +25（3.50x） | P1 | 0 可信 + 2 待核 caller / 7 可信 + 5 待核 callee / 49 unresolved | 待修改 |
| `FN-032` | `func_lines\|internal/control/controller.go\|*Controller.submitCommandOrTurn` | 函数有效行数 | `internal/control/controller.go` | `*Controller.submitCommandOrTurn` | 698 | 127/80 | +47（1.59x） | P1 | 0 可信 + 2 待核 caller / 7 可信 + 5 待核 callee / 49 unresolved | 待修改 |
| `ND-011` | `nesting_depth\|internal/control/controller.go\|*Controller.submitCommandOrTurn` | 嵌套深度 | `internal/control/controller.go` | `*Controller.submitCommandOrTurn` | 698 | 5/4 | +1（1.25x） | P1 | 0 可信 + 2 待核 caller / 7 可信 + 5 待核 callee / 49 unresolved | 待修改 |
| `FL-012` | `file_lines\|internal/control/controller.go\|` | 文件有效行数 | `internal/control/controller.go` | `internal/control/controller.go` | 1 | 3081/800 | +2281（3.85x） | P1 | 0 可信 + 0 待核 caller / 26 可信 + 13 待核 callee / 643 unresolved | 待修改 |
| `CX-213` | `complexity\|internal/control/controller.go\|removeSessionArtifacts` | 圈复杂度 | `internal/control/controller.go` | `removeSessionArtifacts` | 2046 | 12/10 | +2（1.20x） | P3 | 2 可信 + 0 待核 caller / 9 可信 + 0 待核 callee / 5 unresolved | 待修改 |
| `CX-219` | `complexity\|internal/control/controller.go\|*Controller.requestApprovalWithReason` | 圈复杂度 | `internal/control/controller.go` | `*Controller.requestApprovalWithReason` | 3863 | 13/10 | +3（1.30x） | P3 | 1 可信 + 1 待核 caller / 9 可信 + 4 待核 callee / 5 unresolved | 待修改 |
| `CX-215` | `complexity\|internal/control/controller.go\|*Controller.forkNamed` | 圈复杂度 | `internal/control/controller.go` | `*Controller.forkNamed` | 2172 | 11/10 | +1（1.10x） | P3 | 0 可信 + 2 待核 caller / 8 可信 + 4 待核 callee / 17 unresolved | 待修改 |
| `CX-218` | `complexity\|internal/control/controller.go\|listItem` | 圈复杂度 | `internal/control/controller.go` | `listItem` | 3764 | 18/10 | +8（1.80x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 17 unresolved | 待修改 |
| `CX-212` | `complexity\|internal/control/controller.go\|*Controller.ClearSession` | 圈复杂度 | `internal/control/controller.go` | `*Controller.ClearSession` | 1977 | 14/10 | +4（1.40x） | P3 | 0 可信 + 0 待核 caller / 6 可信 + 7 待核 callee / 16 unresolved | 待修改 |
| `CX-210` | `complexity\|internal/control/controller.go\|*Controller.applyPlanExec` | 圈复杂度 | `internal/control/controller.go` | `*Controller.applyPlanExec` | 934 | 14/10 | +4（1.40x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 2 待核 callee / 31 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-209 · 圈复杂度 · `*Controller.submitCommandOrTurn`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-209 |
| Baseline key | `complexity\|internal/control/controller.go\|*Controller.submitCommandOrTurn` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 61） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 698 |
| 函数/方法/包 | `*Controller.submitCommandOrTurn` |
| Baseline / 当前复测 / 阈值 | 35 / 35 / 10 |
| 超出 | +25，3.50x |
| 同一治理对象关联问题 | `FN-032`、`ND-011` |

#### 明确的问题描述

`*Controller.submitCommandOrTurn` 的圈复杂度当前为 **35**，超过限制 **10**（+25，3.50x）。定义范围为 698-833 行；可见控制流热点：L701 `if scopedRefsOnly {`；L726 `case trimmed == "/clear":`；L750 `case strings.HasPrefix(trimmed, "/"):`；L773 `if turn, name, fromTurn, err := ParseBranchTarget(args); err != nil {`；L794 `if err != nil {`；L828 `if c.maybeAutoStartResearchGoal(input, display) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（ParseBranchTarget、parseRewind）、状态与控制（Controller::runRefTurn、Controller::runRefTurnWithRefs、Controller::runGuarded、Controller::maybeAutoStartResearchGoal）、持久化/I-O（FileRefLine）、渲染/输出（Controller::managementNotice）、权限/安全（Controller::runGuarded）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::submitCommandOrTurn（internal/control/controller.go:698-833）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `Controller::submit`（`internal/control/controller.go:658`，调用点 L675，confidence=0.70，instance-method，待源码复核）；`Controller::submitHTTP`（`internal/control/controller.go:678`，调用点 L695，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（7）：** `Controller::runRefTurn`（`internal/control/controller.go:1136`，调用点 L749/L752/L760/L831，confidence=0.90，exact-match，4 条边）；`ParseBranchTarget`（`internal/control/branches.go:14`，调用点 L773，confidence=0.90，exact-match）；`Controller::runRefTurnWithRefs`（`internal/control/controller.go:1147`，调用点 L756，confidence=0.90，exact-match）；`parseRewind`（`internal/control/controller.go:3826`，调用点 L793，confidence=0.90，exact-match）；`FileRefLine`（`internal/control/refs.go:303`，调用点 L751，confidence=0.90，exact-match）；`SlashPathLineRef`（`internal/control/refs.go:318`，调用点 L755，confidence=0.90，exact-match）；`SlashPathLikeLine`（`internal/control/refs.go:335`，调用点 L759，confidence=0.90，exact-match）
- **待源码复核被调候选（5）：** `Controller::runGuarded`（`internal/control/controller.go:468`，调用点 L735/L815/L821，confidence=0.70，instance-method，3 条边，待源码复核）；`Controller::maybeAutoStartResearchGoal`（`internal/control/controller.go:835`，调用点 L828，confidence=0.70，instance-method，待源码复核）；`Controller::applyPlanExec`（`internal/control/controller.go:934`，调用点 L803，confidence=0.70，instance-method，待源码复核）；`Controller::applyPrometheus`（`internal/control/controller.go:1018`，调用点 L806，confidence=0.70，instance-method，待源码复核）；`Controller::managementNotice`（`internal/control/slash.go:420`，调用点 L809，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Controller::SubmitHTTP`（`internal/control/controller.go:641`，调用点 L642，confidence=0.70，instance-method，待源码复核）；`Controller::submit`（`internal/control/controller.go:658`，调用点 L675，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 49 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/control/branches.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.submitCommandOrTurn` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L701、L728、L767、L791、L828 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Controller.submitCommandOrTurn` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-209、FN-032、ND-011 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/controller.go|*Controller.submitCommandOrTurn` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:698-833` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.submitCommandOrTurn` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-209`、`FN-032`、`ND-011` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 race：`go test -race ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/controller.go|*Controller.submitCommandOrTurn` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-032 · 函数有效行数 · `*Controller.submitCommandOrTurn`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-032 |
| Baseline key | `func_lines\|internal/control/controller.go\|*Controller.submitCommandOrTurn` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P2，治理对象得分 61） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 698 |
| 函数/方法/包 | `*Controller.submitCommandOrTurn` |
| Baseline / 当前复测 / 阈值 | 127 / 127 / 80 |
| 超出 | +47，1.59x |
| 同一治理对象关联问题 | `CX-209`、`ND-011` |

#### 明确的问题描述

`*Controller.submitCommandOrTurn` 的有效代码行 **127**，超过限制 **80**（+47）。CodeGraph 定义物理范围为 698-833 行，并记录 12 个直接下游候选；候选职责分布为 配置/解析（ParseBranchTarget、parseRewind）、状态与控制（Controller::runRefTurn、Controller::runRefTurnWithRefs、Controller::runGuarded、Controller::maybeAutoStartResearchGoal）、持久化/I-O（FileRefLine）、渲染/输出（Controller::managementNotice）、权限/安全（Controller::runGuarded）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::submitCommandOrTurn（internal/control/controller.go:698-833）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `Controller::submit`（`internal/control/controller.go:658`，调用点 L675，confidence=0.70，instance-method，待源码复核）；`Controller::submitHTTP`（`internal/control/controller.go:678`，调用点 L695，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（7）：** `Controller::runRefTurn`（`internal/control/controller.go:1136`，调用点 L749/L752/L760/L831，confidence=0.90，exact-match，4 条边）；`ParseBranchTarget`（`internal/control/branches.go:14`，调用点 L773，confidence=0.90，exact-match）；`Controller::runRefTurnWithRefs`（`internal/control/controller.go:1147`，调用点 L756，confidence=0.90，exact-match）；`parseRewind`（`internal/control/controller.go:3826`，调用点 L793，confidence=0.90，exact-match）；`FileRefLine`（`internal/control/refs.go:303`，调用点 L751，confidence=0.90，exact-match）；`SlashPathLineRef`（`internal/control/refs.go:318`，调用点 L755，confidence=0.90，exact-match）；`SlashPathLikeLine`（`internal/control/refs.go:335`，调用点 L759，confidence=0.90，exact-match）
- **待源码复核被调候选（5）：** `Controller::runGuarded`（`internal/control/controller.go:468`，调用点 L735/L815/L821，confidence=0.70，instance-method，3 条边，待源码复核）；`Controller::maybeAutoStartResearchGoal`（`internal/control/controller.go:835`，调用点 L828，confidence=0.70，instance-method，待源码复核）；`Controller::applyPlanExec`（`internal/control/controller.go:934`，调用点 L803，confidence=0.70，instance-method，待源码复核）；`Controller::applyPrometheus`（`internal/control/controller.go:1018`，调用点 L806，confidence=0.70，instance-method，待源码复核）；`Controller::managementNotice`（`internal/control/slash.go:420`，调用点 L809，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Controller::SubmitHTTP`（`internal/control/controller.go:641`，调用点 L642，confidence=0.70，instance-method，待源码复核）；`Controller::submit`（`internal/control/controller.go:658`，调用点 L675，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 49 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/control/branches.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.submitCommandOrTurn` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*Controller.submitCommandOrTurn` 的 127 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L747 `Double-slash — not a command. Common in code snippets (JS`、L763 `Read-only management verbs (/model /memory /skills /hooks /mcp) emit a`、L765 `no extra wiring. (The chat TUI handles these itself with richer output.)`、L813 `turn. (Built-in slash verbs like /compact are handled above.)` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-209、FN-032、ND-011 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/control/controller.go|*Controller.submitCommandOrTurn` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:698-833` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.submitCommandOrTurn` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-209`、`FN-032`、`ND-011` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 race：`go test -race ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/control/controller.go|*Controller.submitCommandOrTurn` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-011 · 嵌套深度 · `*Controller.submitCommandOrTurn`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-011 |
| Baseline key | `nesting_depth\|internal/control/controller.go\|*Controller.submitCommandOrTurn` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P3，治理对象得分 61） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 698 |
| 函数/方法/包 | `*Controller.submitCommandOrTurn` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-209`、`FN-032` |

#### 明确的问题描述

`*Controller.submitCommandOrTurn` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L701 `if scopedRefsOnly {`；L726 `case trimmed == "/clear":`；L750 `case strings.HasPrefix(trimmed, "/"):`；L773 `if turn, name, fromTurn, err := ParseBranchTarget(args); err != nil {`；L794 `if err != nil {`；L828 `if c.maybeAutoStartResearchGoal(input, display) {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::submitCommandOrTurn（internal/control/controller.go:698-833）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `Controller::submit`（`internal/control/controller.go:658`，调用点 L675，confidence=0.70，instance-method，待源码复核）；`Controller::submitHTTP`（`internal/control/controller.go:678`，调用点 L695，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（7）：** `Controller::runRefTurn`（`internal/control/controller.go:1136`，调用点 L749/L752/L760/L831，confidence=0.90，exact-match，4 条边）；`ParseBranchTarget`（`internal/control/branches.go:14`，调用点 L773，confidence=0.90，exact-match）；`Controller::runRefTurnWithRefs`（`internal/control/controller.go:1147`，调用点 L756，confidence=0.90，exact-match）；`parseRewind`（`internal/control/controller.go:3826`，调用点 L793，confidence=0.90，exact-match）；`FileRefLine`（`internal/control/refs.go:303`，调用点 L751，confidence=0.90，exact-match）；`SlashPathLineRef`（`internal/control/refs.go:318`，调用点 L755，confidence=0.90，exact-match）；`SlashPathLikeLine`（`internal/control/refs.go:335`，调用点 L759，confidence=0.90，exact-match）
- **待源码复核被调候选（5）：** `Controller::runGuarded`（`internal/control/controller.go:468`，调用点 L735/L815/L821，confidence=0.70，instance-method，3 条边，待源码复核）；`Controller::maybeAutoStartResearchGoal`（`internal/control/controller.go:835`，调用点 L828，confidence=0.70，instance-method，待源码复核）；`Controller::applyPlanExec`（`internal/control/controller.go:934`，调用点 L803，confidence=0.70，instance-method，待源码复核）；`Controller::applyPrometheus`（`internal/control/controller.go:1018`，调用点 L806，confidence=0.70，instance-method，待源码复核）；`Controller::managementNotice`（`internal/control/slash.go:420`，调用点 L809，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Controller::SubmitHTTP`（`internal/control/controller.go:641`，调用点 L642，confidence=0.70，instance-method，待源码复核）；`Controller::submit`（`internal/control/controller.go:658`，调用点 L675，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 49 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/control/branches.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.submitCommandOrTurn` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 从 L701、L728、L767、L791、L828 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-209、FN-032、ND-011 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/control/controller.go|*Controller.submitCommandOrTurn` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:698-833` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.submitCommandOrTurn` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-209`、`FN-032`、`ND-011` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 race：`go test -race ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/control/controller.go|*Controller.submitCommandOrTurn` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FL-012 · 文件有效行数 · `internal/control/controller.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-012 |
| Baseline key | `file_lines\|internal/control/controller.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 59） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/control/controller.go` |
| Baseline / 当前复测 / 阈值 | 3081 / 3081 / 800 |
| 超出 | +2281，3.85x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/control/controller.go` 当前有效代码行 **3081**，超过限制 **800**（+2281）。CodeGraph 在文件中索引到的主要大符号包括 Controller::submitCommandOrTurn@698-833、Controller::applyPlanExec@934-1011、Controller::RunShell@1063-1132、New@332-400、Controller::forkNamed@2172-2235、Controller::ClearSession@1977-2037；文件同时承载的调用影响覆盖 17 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 223 个函数/方法节点。
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（26）：** `BranchID`（`internal/agent/branch.go:76`，调用点 L2193/L2270/L2305/L2066，confidence=0.90，import，10 条边）；`Agent::SetSession`（`internal/agent/agent.go:444`，调用点 L2220/L2010/L2293/L2347，confidence=0.90，exact-match，9 条边）；`PathFor`（`internal/guardian/guardian.go:216`，调用点 L349/L2224/L2006/L2297，confidence=0.90，import，9 条边）；`NewSession`（`internal/agent/session.go:29`，调用点 L2199/L2010/L2273/L1825，confidence=0.90，import，7 条边）；`ShortGoalForNotice`（`internal/control/goal.go:400`，调用点 L1003/L911/L847/L1031，confidence=0.90，exact-match，5 条边）；`LoadUserExperts`（`internal/expert/user_store.go:121`，调用点 L1750/L1719/L1684/L1661，confidence=0.90，import，5 条边）；`NewSessionPath`（`internal/agent/save.go:536`，调用点 L2202/L2005/L2276/L1956，confidence=0.90，import，4 条边）；`checkpointManager::boundary`（`internal/control/checkpoint.go:77`，调用点 L2182/L2118/L2417/L2238，confidence=0.90，exact-match，4 条边）
- **待源码复核被调候选（13）：** `approvalManager::cancel`（`internal/control/approval.go:98`，调用点 L1092/L3905/L482/L553，confidence=0.70，exact-match，7 条边，待源码复核）；`ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L1112/L3906/L2917/L1365，confidence=0.70，instance-method，4 条边，待源码复核）；`Config::SaveToScope`（`internal/config/edit.go:820`，调用点 L3130/L3003/L3025/L3092，confidence=0.70，instance-method，4 条边，待源码复核）；`ProfilePathResolver::UserConfigPath`（`internal/config/paths.go:176`，调用点 L3130/L3003/L3025/L3092，confidence=0.70，instance-method，4 条边，待源码复核）；`Controller::goalTodos`（`internal/control/goal.go:411`，调用点 L610/L1555/L1567，confidence=0.70，instance-method，3 条边，待源码复核）；`Controller::persistGoalState`（`internal/control/goal.go:421`，调用点 L611/L1556/L1568，confidence=0.70，instance-method，3 条边，待源码复核）；`Selection::State`（`internal/expert/selection.go:59`，调用点 L1829/L1858/L1798，confidence=0.80，instance-method，3 条边，待源码复核）；`Turn::Finish`（`internal/memorycompiler/runtime.go:1728`，调用点 L1997/L2000/L2033，confidence=0.70，instance-method，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 643 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 17 个文件；`internal/control`、`internal/agent`、`internal/guardian`、`internal/expert`、`internal/automation`、`internal/config`、`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/controller_test.go`、`internal/serve/serve_test.go`、`desktop/app_test.go`、`desktop/settings_app_test.go`、`desktop/tabs_topic_test.go`、`desktop/workspace_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/model_test.go`、`internal/cli/statusline_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/agent/branch.go`、`internal/agent/agent.go`、`internal/guardian/guardian.go`、`internal/agent/session.go`、`internal/control/controller_test.go`、`internal/serve/serve_test.go`、`desktop/app_test.go`、`desktop/settings_app_test.go`

#### 待修改项

- [ ] 先冻结 `internal/control/controller.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `Controller::submitCommandOrTurn`、`Controller::applyPlanExec`、`Controller::RunShell`、`New`、`Controller::forkNamed` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-209、CX-210、CX-211、CX-212、CX-213、CX-214、CX-215、CX-216、CX-217、CX-218、CX-219、FN-032、ND-011。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/control/controller.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/control/controller.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/controller_test.go`、`internal/serve/serve_test.go`、`desktop/app_test.go`、`desktop/settings_app_test.go`、`desktop/tabs_topic_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 race：`go test -race ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/control/controller.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-213 · 圈复杂度 · `removeSessionArtifacts`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-213 |
| Baseline key | `complexity\|internal/control/controller.go\|removeSessionArtifacts` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 2046 |
| 函数/方法/包 | `removeSessionArtifacts` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`removeSessionArtifacts` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 2046-2073 行；可见控制流热点：L2047 `if path == "" {`；L2053 `for _, p := range []string{path, agent.BranchMetaPath(path), store.SessionExpert(path), guardian.Pa…`；L2054 `if p == "" {`；L2061 `if dir := ckptDir(path); dir != "" {`；L2062 `if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {`；L2069 `if err := agent.ClearCleanupPending(path); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function removeSessionArtifacts（internal/control/controller.go:2046-2073）
- **可信生产调用方（2）：** `Controller::ClearSession`（`internal/control/controller.go:1977`，调用点 L1996/L2030，confidence=0.90，exact-match，2 条边）；`ReconcileCleanupPending`（`internal/control/controller.go:2077`，调用点 L2079，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（9）：** `BranchID`（`internal/agent/branch.go:76`，调用点 L2066，confidence=0.90，import）；`BranchMetaPath`（`internal/agent/branch.go:87`，调用点 L2053，confidence=0.90，import）；`ClearCleanupPending`（`internal/agent/save.go:244`，调用点 L2069，confidence=0.90，import）；`DeleteSubagentsByParent`（`internal/agent/subagent_store.go:192`，调用点 L2066，confidence=0.90，import）；`ckptDir`（`internal/control/controller.go:440`，调用点 L2061，confidence=0.90，exact-match）；`PathFor`（`internal/guardian/guardian.go:216`，调用点 L2053，confidence=0.90，import）；`CursorPathFor`（`internal/guardian/guardian.go:224`，调用点 L2053，confidence=0.90，import）；`RemoveArtifacts`（`internal/jobs/artifacts.go:26`，调用点 L2050，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::ClearSession`（`internal/control/controller.go:1977`，调用点 L1996，confidence=0.90，exact-match，待源码复核）；`ReconcileCleanupPending`（`internal/control/controller.go:2077`，调用点 L2079，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 5 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 7 个文件；`internal/control`、`internal/agent`、`internal/guardian`、`internal/jobs`、`internal/store`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/agent/branch.go`、`internal/agent/save.go`、`internal/agent/subagent_store.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `removeSessionArtifacts` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L2047、L2053、L2057、L2062、L2069 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `removeSessionArtifacts` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/controller.go|removeSessionArtifacts` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:2046-2073` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `removeSessionArtifacts` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/controller.go|removeSessionArtifacts` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-219 · 圈复杂度 · `*Controller.requestApprovalWithReason`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-219 |
| Baseline key | `complexity\|internal/control/controller.go\|*Controller.requestApprovalWithReason` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 3863 |
| 函数/方法/包 | `*Controller.requestApprovalWithReason` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Controller.requestApprovalWithReason` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 3863-3908 行；可见控制流热点：L3867 `if c.approval.preApproved(tool, subject) {`；L3876 `if c.approval.preApproved(tool, subject) {`；L3893 `select {`；L3894 `case r := <-reply:`；L3900 `if r.allow && r.persist && !requiresFreshApprovalTool(tool) && c.onRemember != nil {`；L3904 `case <-waitCtx.Done():`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（permissionRequestHookPayload）、状态与控制（Controller::newDecisionAudit、Controller::emitRememberResult）、持久化/I-O（ArchiveSink::Err）、渲染/输出（Controller::emitRememberResult）、网络/协议（permissionRequestHookPayload）、权限/安全（approvalManager::preApproved、permissionRequestHookPayload）、任务/并发（approvalManager::waitContext）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::requestApprovalWithReason（internal/control/controller.go:3863-3908）
- **可信生产调用方（1）：** `gateApprover::ApproveWithReason`（`internal/control/controller.go:3450`，调用点 L3463，confidence=0.90，exact-match）
- **待源码复核调用方候选（1）：** `Controller::requestApproval`（`internal/control/controller.go:3859`，调用点 L3860，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（9）：** `approvalManager::preApproved`（`internal/control/approval.go:71`，调用点 L3867/L3876，confidence=0.90，exact-match，2 条边）；`requiresFreshApprovalTool`（`internal/control/approval.go:271`，调用点 L3897/L3900，confidence=0.90，exact-match，2 条边）；`approvalManager::register`（`internal/control/approval.go:79`，调用点 L3880，confidence=0.90，exact-match）；`approvalManager::grantSession`（`internal/control/approval.go:91`，调用点 L3898，confidence=0.90，exact-match）；`approvalManager::waitContext`（`internal/control/approval.go:188`，调用点 L3890，confidence=0.90，exact-match）；`approvalNotificationText`（`internal/control/approval.go:280`，调用点 L3888，confidence=0.90，exact-match）；`permissionRequestHookPayload`（`internal/control/approval.go:290`，调用点 L3883，confidence=0.90，exact-match）；`hookAudit`（`internal/control/controller.go:1448`，调用点 L3884，confidence=0.90，exact-match）
- **待源码复核被调候选（4）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L3906，confidence=0.70，instance-method，待源码复核）；`approvalManager::cancel`（`internal/control/approval.go:98`，调用点 L3905，confidence=0.70，exact-match，待源码复核）；`Controller::newDecisionAudit`（`internal/control/controller.go:1378`，调用点 L3879，confidence=0.70，instance-method，待源码复核）；`Controller::emitRememberResult`（`internal/control/controller.go:3910`，调用点 L3901，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `gateApprover::ApproveWithReason`（`internal/control/controller.go:3450`，调用点 L3463，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 5 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/control`、`internal/permission`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/control/approval.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.requestApprovalWithReason` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L3867、L3883、L3894、L3897、L3904 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Controller.requestApprovalWithReason` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/controller.go|*Controller.requestApprovalWithReason` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:3863-3908` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.requestApprovalWithReason` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 race：`go test -race ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/controller.go|*Controller.requestApprovalWithReason` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-215 · 圈复杂度 · `*Controller.forkNamed`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-215 |
| Baseline key | `complexity\|internal/control/controller.go\|*Controller.forkNamed` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 2172 |
| 函数/方法/包 | `*Controller.forkNamed` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Controller.forkNamed` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 2172-2235 行；可见控制流热点：L2173 `if c.executor == nil {`；L2179 `if c.Running() {`；L2189 `if err := c.Snapshot(); err != nil {`；L2195 `if boundary > len(src) {`；L2207 `if err := agent.SaveBranchMeta(newPath, agent.BranchMeta{`；L2228 `if c.guardianSess != nil {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（Agent::SetSession、Controller::rewindFail、Controller::rebindCheckpoints、Controller::saveExpertMeta）、持久化/I-O（SaveBranchMeta、Controller::saveExpertMeta）、渲染/输出（SessionPreviewFromMessages）、任务/并发（Controller::setActiveJobSession）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::forkNamed（internal/control/controller.go:2172-2235）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `Controller::ForkNamed`（`internal/control/controller.go:2161`，调用点 L2162，confidence=0.70，instance-method，待源码复核）；`Controller::ForkSession`（`internal/control/controller.go:2168`，调用点 L2169，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（8）：** `Agent::SetSession`（`internal/agent/agent.go:444`，调用点 L2220，confidence=0.90，exact-match）；`BranchID`（`internal/agent/branch.go:76`，调用点 L2193，confidence=0.90，import）；`SaveBranchMeta`（`internal/agent/branch.go:113`，调用点 L2207，confidence=0.90，import）；`SessionPreviewFromMessages`（`internal/agent/save.go:468`，调用点 L2206，confidence=0.90，import）；`NewSessionPath`（`internal/agent/save.go:536`，调用点 L2202，confidence=0.90，import）；`NewSession`（`internal/agent/session.go:29`，调用点 L2199，confidence=0.90，import）；`checkpointManager::boundary`（`internal/control/checkpoint.go:77`，调用点 L2182，confidence=0.90，exact-match）；`PathFor`（`internal/guardian/guardian.go:216`，调用点 L2224，confidence=0.90，import）
- **待源码复核被调候选（4）：** `Controller::rewindFail`（`internal/control/controller.go:2100`，调用点 L2174/L2177/L2180/L2184，confidence=0.70，instance-method，6 条边，待源码复核）；`Controller::rebindCheckpoints`（`internal/control/controller.go:447`，调用点 L2227，confidence=0.70，instance-method，待源码复核）；`Controller::saveExpertMeta`（`internal/control/controller.go:1925`，调用点 L2218，confidence=0.70，instance-method，待源码复核）；`Controller::setActiveJobSession`（`internal/control/controller.go:2764`，调用点 L2226，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Controller::ForkNamed`（`internal/control/controller.go:2161`，调用点 L2162，confidence=0.70，instance-method，待源码复核）；`Controller::ForkSession`（`internal/control/controller.go:2168`，调用点 L2169，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 17 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 7 个文件；`internal/control`、`internal/agent`、`internal/guardian`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/agent/agent.go`、`internal/agent/branch.go`、`internal/agent/save.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.forkNamed` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L2173、L2179、L2195、L2207、L2228 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Controller.forkNamed` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/controller.go|*Controller.forkNamed` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:2172-2235` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.forkNamed` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/controller.go|*Controller.forkNamed` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-218 · 圈复杂度 · `listItem`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-218 |
| Baseline key | `complexity\|internal/control/controller.go\|listItem` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 38） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 3764 |
| 函数/方法/包 | `listItem` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`listItem` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 3764-3817 行；可见控制流热点：L3766 `if trimmed == "" {`；L3771 `if c == '\t' {`；L3787 `switch {`；L3793 `for i < len(s) && s[i] >= '0' && s[i] <= '9' {`；L3807 `if s == "" {`；L3813 `if indent >= 2 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function listItem（internal/control/controller.go:3764-3817）
- **可信生产调用方（1）：** `parsePlanTodos`（`internal/control/controller.go:3706`，调用点 L3709，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 17 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `listItem` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L3766、L3783、L3788、L3807、L3813 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `listItem` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/controller.go|listItem` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:3764-3817` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `listItem` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/controller.go|listItem` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-212 · 圈复杂度 · `*Controller.ClearSession`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-212 |
| Baseline key | `complexity\|internal/control/controller.go\|*Controller.ClearSession` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 1977 |
| 函数/方法/包 | `*Controller.ClearSession` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Controller.ClearSession` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 1977-2037 行；可见控制流热点：L1978 `if c.executor == nil {`；L1989 `if preMarkedCleanup {`；L1995 `if !destroy.Async {`；L2012 `if c.guardianSess != nil {`；L2024 `if result.HasTimedOut() && destroy.WaitAll != nil {`；L2030 `if err := removeSessionArtifacts(oldPath); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（Agent::SetSession、Controller::rebindCheckpoints、Controller::saveExpertMeta、Controller::hasUnfinishedSessionJobs）、持久化/I-O（Controller::saveExpertMeta）、任务/并发（Controller::hasUnfinishedSessionJobs、Controller::setActiveJobSession、Manager::Wait）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::ClearSession（internal/control/controller.go:1977-2037）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `MarkCleanupPending`（`internal/agent/save.go:227`，调用点 L1990/L2025，confidence=0.90，import，2 条边）；`removeSessionArtifacts`（`internal/control/controller.go:2046`，调用点 L1996/L2030，confidence=0.90，exact-match，2 条边）；`Agent::SetSession`（`internal/agent/agent.go:444`，调用点 L2010，confidence=0.90，exact-match）；`NewSessionPath`（`internal/agent/save.go:536`，调用点 L2005，confidence=0.90，import）；`NewSession`（`internal/agent/session.go:29`，调用点 L2010，confidence=0.90，import）；`PathFor`（`internal/guardian/guardian.go:216`，调用点 L2006，confidence=0.90，import）
- **待源码复核被调候选（7）：** `Turn::Finish`（`internal/memorycompiler/runtime.go:1728`，调用点 L1997/L2000/L2033，confidence=0.70，instance-method，3 条边，待源码复核）；`Controller::rebindCheckpoints`（`internal/control/controller.go:447`，调用点 L2016，confidence=0.70，instance-method，待源码复核）；`Controller::saveExpertMeta`（`internal/control/controller.go:1925`，调用点 L2011，confidence=0.70，instance-method，待源码复核）；`Controller::hasUnfinishedSessionJobs`（`internal/control/controller.go:2039`，调用点 L1988，confidence=0.70，instance-method，待源码复核）；`Controller::setActiveJobSession`（`internal/control/controller.go:2764`，调用点 L2009，confidence=0.70，instance-method，待源码复核）；`TeardownResult::HasTimedOut`（`internal/jobs/jobs.go:77`，调用点 L2024，confidence=0.80，instance-method，待源码复核）；`Manager::Wait`（`internal/jobs/jobs.go:702`，调用点 L2023，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 16 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 7 个文件；`internal/control`、`internal/agent`、`internal/guardian`、`internal/memorycompiler`、`internal/jobs`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/agent/save.go`、`internal/agent/agent.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.ClearSession` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1978、L1990、L2003、L2021、L2030 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Controller.ClearSession` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/controller.go|*Controller.ClearSession` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:1977-2037` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.ClearSession` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 race：`go test -race ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/controller.go|*Controller.ClearSession` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-210 · 圈复杂度 · `*Controller.applyPlanExec`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-210 |
| Baseline key | `complexity\|internal/control/controller.go\|*Controller.applyPlanExec` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 934 |
| 函数/方法/包 | `*Controller.applyPlanExec` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Controller.applyPlanExec` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 934-1011 行；可见控制流热点：L936 `if len(todos) == 0 {`；L945 `if f == "--strict" {`；L965 `if len(modules) > 0 {`；L974 `for _, t := range todos {`；L992 `if done > 0 {`；L1006 `if c.runner != nil {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（Agent::CanonicalTodoState、Controller::runGuarded、Controller::detectProjectModules）、渲染/输出（ShortGoalForNotice）、权限/安全（Controller::runGuarded）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::applyPlanExec（internal/control/controller.go:934-1011）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Controller::submitCommandOrTurn`（`internal/control/controller.go:698`，调用点 L803，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `Agent::CanonicalTodoState`（`internal/agent/agent.go:1016`，调用点 L935，confidence=0.90，exact-match）；`ShortGoalForNotice`（`internal/control/goal.go:400`，调用点 L1003，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Controller::runGuarded`（`internal/control/controller.go:468`，调用点 L1007，confidence=0.70，instance-method，待源码复核）；`Controller::detectProjectModules`（`internal/control/controller.go:3930`，调用点 L964，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Controller::submit`（`internal/control/controller.go:658`，调用点 L675，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 31 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/control`、`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/agent/agent.go`、`internal/control/goal.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.applyPlanExec` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L936、L954、L967、L980、L1006 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、渲染/输出、权限/安全 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Controller.applyPlanExec` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/controller.go|*Controller.applyPlanExec` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:934-1011` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.applyPlanExec` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G31 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/controller.go|*Controller.applyPlanExec` 下降或消失且无新增/恶化。
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
