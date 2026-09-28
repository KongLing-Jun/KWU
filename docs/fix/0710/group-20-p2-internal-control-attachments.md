# G20 · internal/control · attachments 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G20 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/control` |
| 涉及文件 | `internal/control/attachments.go`<br>`internal/control/profile_management.go`<br>`internal/control/input.go`<br>`internal/control/turn_orchestrator.go`<br>`internal/control/branches.go` |
| 主要治理目标 | 围绕 `internal/control/attachments.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | control 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 5 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 20 组；无同文件组依赖 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 持久化、迁移或文件格式兼容；返回值、错误语义和现有调用顺序；配置、平台或向后兼容；并发、取消或事件顺序；权限、安全或凭据边界；会话、状态或恢复语义 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **48/100**，对象 `SaveAttachmentFile` 的分项为严重度 9.5、集中度 5.9、影响面 20.4、可执行性 12.0；最高超限 2.00x，文件内 5 条问题，CodeGraph 记录 1 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 5 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-201` | `complexity\|internal/control/attachments.go\|SaveAttachmentFile` | 圈复杂度 | `internal/control/attachments.go` | `SaveAttachmentFile` | 159 | 20/10 | +10（2.00x） | P2 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-224` | `complexity\|internal/control/profile_management.go\|*Controller.profileCommandText` | 圈复杂度 | `internal/control/profile_management.go` | `*Controller.profileCommandText` | 79 | 21/10 | +11（2.10x） | P2 | 0 可信 + 1 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-202` | `complexity\|internal/control/attachments.go\|readAttachmentImage` | 圈复杂度 | `internal/control/attachments.go` | `readAttachmentImage` | 290 | 17/10 | +7（1.70x） | P2 | 2 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-223` | `complexity\|internal/control/input.go\|shouldAutoStartResearchGoal` | 圈复杂度 | `internal/control/input.go` | `shouldAutoStartResearchGoal` | 281 | 18/10 | +8（1.80x） | P2 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-237` | `complexity\|internal/control/turn_orchestrator.go\|*turnOrchestrator.runTurnWithRawDisplay` | 圈复杂度 | `internal/control/turn_orchestrator.go` | `*turnOrchestrator.runTurnWithRawDisplay` | 22 | 16/10 | +6（1.60x） | P2 | 0 可信 + 1 待核 caller / 8 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `CX-199` | `complexity\|internal/control/attachments.go\|SaveImageBytes` | 圈复杂度 | `internal/control/attachments.go` | `SaveImageBytes` | 86 | 11/10 | +1（1.10x） | P2 | 5 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-200` | `complexity\|internal/control/attachments.go\|SaveImageFile` | 圈复杂度 | `internal/control/attachments.go` | `SaveImageFile` | 121 | 15/10 | +5（1.50x） | P2 | 2 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-207` | `complexity\|internal/control/branches.go\|FormatBranchTree` | 圈复杂度 | `internal/control/branches.go` | `FormatBranchTree` | 39 | 12/10 | +2（1.20x） | P2 | 2 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-208` | `complexity\|internal/control/branches.go\|shortBranchID` | 圈复杂度 | `internal/control/branches.go` | `shortBranchID` | 143 | 11/10 | +1（1.10x） | P2 | 2 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-198` | `complexity\|internal/control/attachments.go\|SaveAttachmentDataURL` | 圈复杂度 | `internal/control/attachments.go` | `SaveAttachmentDataURL` | 32 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-201 · 圈复杂度 · `SaveAttachmentFile`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-201 |
| Baseline key | `complexity\|internal/control/attachments.go\|SaveAttachmentFile` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 48） |
| 文件 | `internal/control/attachments.go` |
| 代码行 | 159 |
| 函数/方法/包 | `SaveAttachmentFile` |
| Baseline / 当前复测 / 阈值 | 20 / 20 / 10 |
| 超出 | +10，2.00x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`SaveAttachmentFile` 的圈复杂度当前为 **20**，超过限制 **10**（+10，2.00x）。定义范围为 159-215 行；可见控制流热点：L161 `if err != nil {`；L171 `if err != nil {`；L179 `if !os.SameFile(info, opened) {`；L189 `if after, err := f.Stat(); err != nil {`；L198 `if err := ensureAttachmentRoot(); err != nil {`；L210 `if err := dst.Close(); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（createAttachmentFile）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function SaveAttachmentFile（internal/control/attachments.go:159-215）
- **可信生产调用方（1）：** `App::AttachDropped`（`desktop/app.go:7062`，调用点 L7083，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `ensureAttachmentRoot`（`internal/control/attachments.go:379`，调用点 L198，confidence=0.90，exact-match）；`createAttachmentFile`（`internal/control/attachments.go:462`，调用点 L201，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::AttachDropped`（`desktop/app.go:7062`，调用点 L7083，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`、`desktop`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/attachments_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/attachments.go`、`desktop/app.go`、`internal/control/attachments_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `SaveAttachmentFile` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L161、L171、L186、L198、L210 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `SaveAttachmentFile` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/attachments.go|SaveAttachmentFile` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/attachments.go:159-215` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `SaveAttachmentFile` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/attachments_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/attachments.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/attachments.go|SaveAttachmentFile` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-224 · 圈复杂度 · `*Controller.profileCommandText`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-224 |
| Baseline key | `complexity\|internal/control/profile_management.go\|*Controller.profileCommandText` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 48） |
| 文件 | `internal/control/profile_management.go` |
| 代码行 | 79 |
| 函数/方法/包 | `*Controller.profileCommandText` |
| Baseline / 当前复测 / 阈值 | 21 / 21 / 10 |
| 超出 | +11，2.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Controller.profileCommandText` 的圈复杂度当前为 **21**，超过限制 **10**（+11，2.10x）。定义范围为 79-149 行；可见控制流热点：L80 `if len(fields) == 0 {`；L92 `if err != nil {`；L105 `if err != nil {`；L119 `case "delete", "rm":`；L129 `if len(args) != 1 {`；L142 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（profileListText、profileListScope、profileDetailText）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::profileCommandText（internal/control/profile_management.go:79-149）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Controller::profileNotice`（`internal/control/profile_management.go:74`，调用点 L75，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `profileListText`（`internal/control/profile_management.go:168`，调用点 L99/L118，confidence=0.90，exact-match，2 条边）；`profileListScope`（`internal/control/profile_management.go:151`，调用点 L91，confidence=0.90，exact-match）；`profileDetailText`（`internal/control/profile_management.go:180`，调用点 L108，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/profile_management.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.profileCommandText` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L80、L96、L111、L128、L142 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Controller.profileCommandText` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/profile_management.go|*Controller.profileCommandText` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/profile_management.go:79-149` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.profileCommandText` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/profile_management.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/profile_management.go|*Controller.profileCommandText` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-202 · 圈复杂度 · `readAttachmentImage`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-202 |
| Baseline key | `complexity\|internal/control/attachments.go\|readAttachmentImage` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 46） |
| 文件 | `internal/control/attachments.go` |
| 代码行 | 290 |
| 函数/方法/包 | `readAttachmentImage` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`readAttachmentImage` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 290-334 行；可见控制流热点：L292 `if err != nil {`；L299 `if info.Mode()&os.ModeSymlink != 0 {`；L306 `if err != nil {`；L314 `if !os.SameFile(info, opened) {`；L321 `if len(raw) == 0 || len(raw) > maxImageAttachmentBytes {`；L330 `if mime == "" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function readAttachmentImage（internal/control/attachments.go:290-334）
- **可信生产调用方（2）：** `ImageDataURL`（`internal/control/attachments.go:269`，调用点 L270，confidence=0.90，exact-match）；`visionImageDataURL`（`internal/control/attachments.go:281`，调用点 L282，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `cleanAttachmentPath`（`internal/control/attachments.go:336`，调用点 L291，confidence=0.90，exact-match）；`detectedImageMime`（`internal/control/attachments.go:483`，调用点 L329，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::AttachmentDataURL`（`desktop/app.go:7042`，调用点 L7044，confidence=0.90，import，待源码复核）；`App::AttachDropped`（`desktop/app.go:7062`，调用点 L7071，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/attachments.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `readAttachmentImage` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L292、L302、L311、L321、L330 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `readAttachmentImage` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/attachments.go|readAttachmentImage` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/attachments.go:290-334` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `readAttachmentImage` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/attachments.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/attachments.go|readAttachmentImage` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-223 · 圈复杂度 · `shouldAutoStartResearchGoal`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-223 |
| Baseline key | `complexity\|internal/control/input.go\|shouldAutoStartResearchGoal` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 46） |
| 文件 | `internal/control/input.go` |
| 代码行 | 281 |
| 函数/方法/包 | `shouldAutoStartResearchGoal` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`shouldAutoStartResearchGoal` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 281-309 行；可见控制流热点：L283 `if trimmed == "" || strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "!") {`；L290 `for _, phrase := range autoResearchAutoStartPhrases {`；L296 `switch {`；L297 `case strings.Contains(lower, "彻底") && categories >= 3:`；L301 `case strings.Contains(lower, "长期") && categories >= 2 && containsAnyGoalKeyword(lower, []string{"实验…`；L305 `case strings.Contains(lower, "complete") && categories >= 3:`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function shouldAutoStartResearchGoal（internal/control/input.go:281-309）
- **可信生产调用方（1）：** `Controller::autoStartResearchGoalCandidate`（`internal/control/controller.go:874`，调用点 L876，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `autoResearchPhaseCount`（`internal/control/input.go:328`，调用点 L295，confidence=0.90，exact-match）；`containsAnyGoalKeyword`（`internal/control/input.go:405`，调用点 L301，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::AutoStartResearchGoal`（`internal/control/controller.go:864`，调用点 L865，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/input_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`
- **需同步检查：** `internal/control/input.go`、`internal/control/controller.go`、`internal/control/input_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`

#### 待修改项

- [ ] 先冻结 `shouldAutoStartResearchGoal` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L283、L290、L297、L301、L305 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `shouldAutoStartResearchGoal` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/input.go|shouldAutoStartResearchGoal` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/input.go:281-309` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `shouldAutoStartResearchGoal` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/input_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/input.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/input.go|shouldAutoStartResearchGoal` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-237 · 圈复杂度 · `*turnOrchestrator.runTurnWithRawDisplay`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-237 |
| Baseline key | `complexity\|internal/control/turn_orchestrator.go\|*turnOrchestrator.runTurnWithRawDisplay` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 46） |
| 文件 | `internal/control/turn_orchestrator.go` |
| 代码行 | 22 |
| 函数/方法/包 | `*turnOrchestrator.runTurnWithRawDisplay` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*turnOrchestrator.runTurnWithRawDisplay` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 22-112 行；可见控制流热点：L43 `if c.guardianSess != nil {`；L54 `if block, _ := c.hooks.PromptSubmit(ctx, input, turn); block {`；L67 `if errors.Is(err, context.Canceled) && c.CancelRequested() {`；L86 `if err != nil {`；L102 `if err = c.runner.Run(ctx, c.ComposeSynthetic(planApprovedMessage)); err != nil {`；L108 `if todoArgs != "" && !c.hasTodoUpdateSince(execStart) {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（approvalManager::setPlanAutoApprove、Session::ResetTurn、Runner::Enabled、Runner::PromptSubmit）、持久化/I-O（Controller::enqueueProfileLearningObservation、Controller::profileLearningObservation）、权限/安全（approvalManager::setPlanAutoApprove）、任务/并发（Controller::enqueueProfileLearningObservation）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method turnOrchestrator::runTurnWithRawDisplay（internal/control/turn_orchestrator.go:22-112）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Controller::runTurnWithRawDisplay`（`internal/control/controller.go:589`，调用点 L590，confidence=0.85，instance-method，待源码复核）
- **可信被调用函数（8）：** `approvalManager::setPlanAutoApprove`（`internal/control/approval.go:181`，调用点 L100/L101，confidence=0.90，exact-match，2 条边）；`WithParentSession`（`internal/agent/agent.go:106`，调用点 L27，confidence=0.90，import）；`WithUserImages`（`internal/agent/agent.go:120`，调用点 L29，confidence=0.90，import）；`WithMemoryCompilerSourceInput`（`internal/agent/memory_compiler_context.go:14`，调用点 L30，confidence=0.90，import）；`Session::ResetTurn`（`internal/guardian/guardian.go:329`，调用点 L44，confidence=0.90，exact-match）；`Runner::Enabled`（`internal/hook/runner.go:37`，调用点 L49，confidence=0.90，exact-match）；`Runner::PromptSubmit`（`internal/hook/runner.go:95`，调用点 L54，confidence=0.90，exact-match）；`WithSession`（`internal/jobs/jobs.go:1492`，调用点 L28，confidence=0.90，import）
- **待源码复核被调候选（4）：** `Controller::maybeAutoPlan`（`internal/control/auto_plan.go:40`，调用点 L25，confidence=0.70，instance-method，待源码复核）；`Controller::enqueueProfileLearningObservation`（`internal/control/profile_learning.go:13`，调用点 L36，confidence=0.70，instance-method，待源码复核）；`Controller::profileLearningObservation`（`internal/control/profile_learning.go:25`，调用点 L36，confidence=0.70，instance-method，待源码复核）；`Controller::inputImages`（`internal/control/refs.go:156`，调用点 L29，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Controller::runTurnWithRawDisplay`（`internal/control/controller.go:589`，调用点 L590，confidence=0.85，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 11 个文件；`internal/control`、`internal/agent`、`internal/guardian`、`internal/hook`、`internal/jobs`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/turn_orchestrator.go`、`internal/control/controller.go`、`internal/control/approval.go`、`internal/agent/agent.go`、`internal/agent/memory_compiler_context.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*turnOrchestrator.runTurnWithRawDisplay` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L43、L59、L79、L89、L108 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O、权限/安全 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*turnOrchestrator.runTurnWithRawDisplay` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/turn_orchestrator.go|*turnOrchestrator.runTurnWithRawDisplay` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/turn_orchestrator.go:22-112` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*turnOrchestrator.runTurnWithRawDisplay` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 race：`go test -race ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/turn_orchestrator.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/turn_orchestrator.go|*turnOrchestrator.runTurnWithRawDisplay` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-199 · 圈复杂度 · `SaveImageBytes`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-199 |
| Baseline key | `complexity\|internal/control/attachments.go\|SaveImageBytes` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 45） |
| 文件 | `internal/control/attachments.go` |
| 代码行 | 86 |
| 函数/方法/包 | `SaveImageBytes` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`SaveImageBytes` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 86-119 行；可见控制流热点：L87 `if len(raw) == 0 || len(raw) > maxImageAttachmentBytes {`；L91 `if mime == "" {`；L94 `if declaredMime != "" && imageExt(declaredMime) == "" {`；L102 `if err != nil {`；L105 `if n, err := f.Write(raw); err != nil {`；L114 `if err := f.Close(); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（createAttachmentFile）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function SaveImageBytes（internal/control/attachments.go:86-119）
- **可信生产调用方（5）：** `SaveImageDataURL`（`internal/control/attachments.go:68`，调用点 L83，confidence=0.90，exact-match）；`SaveImageFile`（`internal/control/attachments.go:121`，调用点 L156，confidence=0.90，exact-match）；`saveWindowsClipboardImage`（`internal/control/attachments.go:230`，调用点 L253，confidence=0.90，exact-match）；`saveLinuxClipboardImage`（`internal/control/attachments.go:256`，调用点 L263，confidence=0.90，exact-match）；`saveDarwinClipboardClass`（`internal/control/attachments.go:414`，调用点 L459，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `imageExt`（`internal/control/attachments.go:494`，调用点 L94/L97，confidence=0.90，exact-match，2 条边）；`ensureAttachmentRoot`（`internal/control/attachments.go:379`，调用点 L98，confidence=0.90，exact-match）；`createAttachmentFile`（`internal/control/attachments.go:462`，调用点 L101，confidence=0.90，exact-match）；`detectedImageMime`（`internal/control/attachments.go:483`，调用点 L90，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::SavePastedImage`（`desktop/app.go:6867`，调用点 L6869，confidence=0.90，import，待源码复核）；`App::AttachDropped`（`desktop/app.go:7062`，调用点 L7070，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/attachments_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/attachments.go`、`internal/control/attachments_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `SaveImageBytes` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L87、L94、L98、L105、L114 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `SaveImageBytes` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/attachments.go|SaveImageBytes` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/attachments.go:86-119` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `SaveImageBytes` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/attachments_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/attachments.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/attachments.go|SaveImageBytes` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-200 · 圈复杂度 · `SaveImageFile`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-200 |
| Baseline key | `complexity\|internal/control/attachments.go\|SaveImageFile` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 45） |
| 文件 | `internal/control/attachments.go` |
| 代码行 | 121 |
| 函数/方法/包 | `SaveImageFile` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`SaveImageFile` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 121-157 行；可见控制流热点：L123 `if err != nil {`；L129 `if info.IsDir() || info.Size() <= 0 || info.Size() > maxImageAttachmentBytes {`；L133 `if err != nil {`；L141 `if !os.SameFile(info, opened) {`；L145 `if err != nil {`；L151 `if after, err := f.Stat(); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（SaveImageBytes）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function SaveImageFile（internal/control/attachments.go:121-157）
- **可信生产调用方（2）：** `App::AttachDropped`（`desktop/app.go:7062`，调用点 L7070，confidence=0.90，import）；`savePastedImageSource`（`internal/cli/chat_tui_paste.go:219`，调用点 L228，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `SaveImageBytes`（`internal/control/attachments.go:86`，调用点 L156，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::AttachDropped`（`desktop/app.go:7062`，调用点 L7070，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/control`、`desktop`、`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/attachments_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/attachments.go`、`desktop/app.go`、`internal/cli/chat_tui_paste.go`、`internal/control/attachments_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `SaveImageFile` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L123、L129、L138、L145、L151 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `SaveImageFile` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/attachments.go|SaveImageFile` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/attachments.go:121-157` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `SaveImageFile` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/attachments_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/attachments.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/attachments.go|SaveImageFile` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-207 · 圈复杂度 · `FormatBranchTree`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-207 |
| Baseline key | `complexity\|internal/control/branches.go\|FormatBranchTree` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 45） |
| 文件 | `internal/control/branches.go` |
| 代码行 | 39 |
| 函数/方法/包 | `FormatBranchTree` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`FormatBranchTree` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 39-92 行；可见控制流热点：L40 `if len(branches) == 0 {`；L49 `for _, b := range branches {`；L54 `if _, ok := byID[b.ParentID]; !ok {`；L71 `if last {`；L81 `for i, child := range children[b.ID] {`；L88 `for _, b := range branches {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function FormatBranchTree（internal/control/branches.go:39-92）
- **可信生产调用方（2）：** `chatTUI::showBranchTree`（`internal/cli/branch.go:13`，调用点 L20，confidence=0.90，import）；`Controller::BranchTreeText`（`internal/control/branches.go:31`，调用点 L36，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `branchTitle`（`internal/control/branches.go:94`，调用点 L80，confidence=0.90，exact-match）；`turnText`（`internal/control/branches.go:136`，调用点 L80，confidence=0.90，exact-match）；`shortBranchID`（`internal/control/branches.go:143`，调用点 L80，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::BranchTreeText`（`internal/control/branches.go:31`，调用点 L36，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`、`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/branches_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/branches.go`、`internal/cli/branch.go`、`internal/control/branches_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`

#### 待修改项

- [ ] 先冻结 `FormatBranchTree` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L40、L50、L65、L81、L88 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `FormatBranchTree` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/branches.go|FormatBranchTree` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/branches.go:39-92` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `FormatBranchTree` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/branches_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/branches.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/branches.go|FormatBranchTree` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-208 · 圈复杂度 · `shortBranchID`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-208 |
| Baseline key | `complexity\|internal/control/branches.go\|shortBranchID` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 44） |
| 文件 | `internal/control/branches.go` |
| 代码行 | 143 |
| 函数/方法/包 | `shortBranchID` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`shortBranchID` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 143-155 行；可见控制流热点：L144 `if len(id) >= 16 && numeric(id[:8]) && id[8] == '-' && numeric(id[9:15]) && id[15] == '.' {`；L146 `for fracEnd < len(id) && fracEnd < 19 && id[fracEnd] >= '0' && id[fracEnd] <= '9' {`；L149 `if fracEnd > 16 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function shortBranchID（internal/control/branches.go:143-155）
- **可信生产调用方（2）：** `FormatBranchTree`（`internal/control/branches.go:39`，调用点 L80，confidence=0.90，exact-match）；`resolveBranch`（`internal/control/controller.go:2362`，调用点 L2374，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `numeric`（`internal/control/branches.go:157`，调用点 L144，confidence=0.90，exact-match，2 条边）；`oneLineBranch`（`internal/control/branches.go:166`，调用点 L154，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::BranchTreeText`（`internal/control/branches.go:31`，调用点 L36，confidence=0.90，exact-match，待源码复核）；`Controller::SwitchBranch`（`internal/control/controller.go:2320`，调用点 L2335，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/branches_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/branches.go`、`internal/control/controller.go`、`internal/control/branches_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`

#### 待修改项

- [ ] 先冻结 `shortBranchID` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L144、L146、L149 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `shortBranchID` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/branches.go|shortBranchID` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/branches.go:143-155` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `shortBranchID` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/branches_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/branches.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/branches.go|shortBranchID` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-198 · 圈复杂度 · `SaveAttachmentDataURL`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-198 |
| Baseline key | `complexity\|internal/control/attachments.go\|SaveAttachmentDataURL` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/control/attachments.go` |
| 代码行 | 32 |
| 函数/方法/包 | `SaveAttachmentDataURL` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`SaveAttachmentDataURL` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 32-66 行；可见控制流热点：L35 `if !strings.HasPrefix(dataURL, "data:") || i < 0 {`；L39 `if err != nil {`；L46 `if !safeAttachmentExt.MatchString(ext) {`；L49 `if err := ensureAttachmentRoot(); err != nil {`；L56 `if _, err := f.Write(raw); err != nil {`；L61 `if err := f.Close(); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（createAttachmentFile）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function SaveAttachmentDataURL（internal/control/attachments.go:32-66）
- **可信生产调用方（1）：** `App::SavePastedFile`（`desktop/app.go:6882`，调用点 L6884，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `ensureAttachmentRoot`（`internal/control/attachments.go:379`，调用点 L49，confidence=0.90，exact-match）；`createAttachmentFile`（`internal/control/attachments.go:462`，调用点 L52，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::SavePastedFile`（`desktop/app.go:6882`，调用点 L6884，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`、`desktop`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/attachments.go`、`desktop/app.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `SaveAttachmentDataURL` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L35、L42、L49、L53、L61 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `SaveAttachmentDataURL` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/attachments.go|SaveAttachmentDataURL` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/attachments.go:32-66` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `SaveAttachmentDataURL` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/attachments.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/attachments.go|SaveAttachmentDataURL` 下降或消失且无新增/恶化。
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
