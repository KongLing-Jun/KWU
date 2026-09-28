# G31 · Controller 会话控制与提交链路

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G31 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/control`、`internal/agent` |
| 涉及文件 | `internal/control/errmsg.go`<br>`internal/control/imagecompress.go`<br>`internal/control/refs.go`<br>`internal/agent/ask.go`<br>`internal/agent/save.go`<br>`internal/control/auto_plan_classifier.go`<br>`internal/control/controller.go` |
| 主要治理目标 | 围绕 `internal/control/controller.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | control / agent 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614`、`origin/expert-202607081622`；单文件最多集中 14 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 31 组；等待 G06、G16、G18 的同文件变更合并 |
| 前置依赖 | G06（internal/control/controller.go）；G16（internal/agent/save.go）；G18（internal/control/refs.go） |
| 后续验证重点 | 协议、序列化或流式事件兼容；会话、状态或恢复语义；持久化、迁移或文件格式兼容；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **44/100**，对象 `explainError` 的分项为严重度 2.9、集中度 2.3、影响面 22.7、可执行性 16.0；最高超限 1.20x，文件内 1 条问题，CodeGraph 记录 2 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 7 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G06、G16、G18。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-220` | `complexity\|internal/control/errmsg.go\|explainError` | 圈复杂度 | `internal/control/errmsg.go` | `explainError` | 15 | 12/10 | +2（1.20x） | P2 | 2 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-222` | `complexity\|internal/control/imagecompress.go\|compressForVision` | 圈复杂度 | `internal/control/imagecompress.go` | `compressForVision` | 28 | 11/10 | +1（1.10x） | P3 | 2 可信 + 0 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-230` | `complexity\|internal/control/refs.go\|walkRootDir` | 圈复杂度 | `internal/control/refs.go` | `walkRootDir` | 635 | 11/10 | +1（1.10x） | P3 | 2 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-025` | `complexity\|internal/agent/ask.go\|*AskTool.Execute` | 圈复杂度 | `internal/agent/ask.go` | `*AskTool.Execute` | 71 | 12/10 | +2（1.20x） | P3 | 0 可信 + 0 待核 caller / 2 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-046` | `complexity\|internal/agent/save.go\|ListCleanupPending` | 圈复杂度 | `internal/agent/save.go` | `ListCleanupPending` | 273 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-203` | `complexity\|internal/control/auto_plan_classifier.go\|*ProviderAutoPlanClassifier.NeedsPlan` | 圈复杂度 | `internal/control/auto_plan_classifier.go` | `*ProviderAutoPlanClassifier.NeedsPlan` | 42 | 13/10 | +3（1.30x） | P3 | 0 可信 + 0 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-214` | `complexity\|internal/control/controller.go\|*Controller.Rewind` | 圈复杂度 | `internal/control/controller.go` | `*Controller.Rewind` | 2111 | 14/10 | +4（1.40x） | P3 | 0 可信 + 0 待核 caller / 4 可信 + 1 待核 callee / 15 unresolved | 待修改 |
| `CX-211` | `complexity\|internal/control/controller.go\|*Controller.DeleteUserExpert` | 圈复杂度 | `internal/control/controller.go` | `*Controller.DeleteUserExpert` | 1706 | 12/10 | +2（1.20x） | P3 | 0 可信 + 0 待核 caller / 3 可信 + 1 待核 callee / 8 unresolved | 待修改 |
| `CX-216` | `complexity\|internal/control/controller.go\|resolveBranch` | 圈复杂度 | `internal/control/controller.go` | `resolveBranch` | 2362 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 15 unresolved | 待修改 |
| `CX-217` | `complexity\|internal/control/controller.go\|*Controller.maybeColdResumePrune` | 圈复杂度 | `internal/control/controller.go` | `*Controller.maybeColdResumePrune` | 2538 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 0 待核 callee / 7 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-220 · 圈复杂度 · `explainError`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-220 |
| Baseline key | `complexity\|internal/control/errmsg.go\|explainError` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 44） |
| 文件 | `internal/control/errmsg.go` |
| 代码行 | 15 |
| 函数/方法/包 | `explainError` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`explainError` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 15-54 行；可见控制流热点：L16 `if err == nil {`；L22 `if provider.IsConnReset(err) {`；L28 `if msg == "" {`；L37 `if errors.As(err, &authErr) {`；L42 `if authErr.KeyEnv != "" {`；L46 `if authErr.KeyOwner != "" {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（IsConnReset）、网络/协议（requestErrorReason、Messages::ProviderStatusMessage、IsStreamInterrupted）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function explainError（internal/control/errmsg.go:15-54）
- **可信生产调用方（2）：** `Controller::runGuarded`（`internal/control/controller.go:468`，调用点 L499，confidence=0.90，exact-match）；`Controller::RunTurnDisplay`（`internal/control/controller.go:548`，调用点 L572，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `requestErrorReason`（`internal/control/errmsg.go:59`，调用点 L31，confidence=0.90，exact-match）；`Messages::ProviderStatusMessage`（`internal/i18n/i18n.go:415`，调用点 L27，confidence=0.90，exact-match）；`IsStreamInterrupted`（`internal/provider/provider.go:584`，调用点 L19，confidence=0.90，import）；`IsConnReset`（`internal/provider/retry.go:105`，调用点 L22，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::SendWithRaw`（`internal/control/controller.go:514`，调用点 L515，confidence=0.70，instance-method，待源码复核）；`Controller::RunTurnDisplay`（`internal/control/controller.go:548`，调用点 L572，confidence=0.90，exact-match，待源码复核）；`Controller::RunShell`（`internal/control/controller.go:1063`，调用点 L1069，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/control`、`internal/i18n`、`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/errmsg_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/errmsg.go`、`internal/control/controller.go`、`internal/i18n/i18n.go`、`internal/provider/provider.go`、`internal/provider/retry.go`、`internal/control/errmsg_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`

#### 待修改项

- [ ] 先冻结 `explainError` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L16、L26、L31、L42、L46 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `explainError` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/errmsg.go|explainError` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/errmsg.go:15-54` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `explainError` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/errmsg_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/errmsg.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/errmsg.go|explainError` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-222 · 圈复杂度 · `compressForVision`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-222 |
| Baseline key | `complexity\|internal/control/imagecompress.go\|compressForVision` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/control/imagecompress.go` |
| 代码行 | 28 |
| 函数/方法/包 | `compressForVision` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`compressForVision` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 28-60 行；可见控制流热点：L29 `switch mime {`；L30 `case "image/png", "image/jpeg", "image/gif", "image/webp":`；L38 `if cfg.Width <= maxVisionDim && cfg.Height <= maxVisionDim {`；L42 `if err != nil {`；L51 `if err := png.Encode(&buf, dst); err != nil {`；L56 `if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（Store::Bounds）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function compressForVision（internal/control/imagecompress.go:28-60）
- **可信生产调用方（2）：** `visionImageDataURL`（`internal/control/attachments.go:281`，调用点 L286，confidence=0.90，exact-match）；`dataURLFromImageReader`（`internal/control/refs.go:222`，调用点 L234，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `scaledDims`（`internal/control/imagecompress.go:64`，调用点 L45，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Store::Bounds`（`internal/checkpoint/checkpoint.go:116`，调用点 L47，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/control`、`internal/checkpoint`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/control/imagecompress_test.go`
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/imagecompress.go`、`internal/control/attachments.go`、`internal/control/refs.go`、`internal/checkpoint/checkpoint.go`、`internal/control/imagecompress_test.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`

#### 待修改项

- [ ] 先冻结 `compressForVision` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L29、L35、L42、L50、L56 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `compressForVision` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/imagecompress.go|compressForVision` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/imagecompress.go:28-60` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `compressForVision` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/control/imagecompress_test.go`。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/imagecompress.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/imagecompress.go|compressForVision` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-230 · 圈复杂度 · `walkRootDir`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-230 |
| Baseline key | `complexity\|internal/control/refs.go\|walkRootDir` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/control/refs.go` |
| 代码行 | 635 |
| 函数/方法/包 | `walkRootDir` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`walkRootDir` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 635-672 行；可见控制流热点：L636 `if depth > 16 || *n >= maxDirEntries {`；L645 `if err != nil {`；L649 `if *n >= maxDirEntries {`；L654 `if e.IsDir() {`；L656 `case ".git", "node_modules", ".DS_Store", "__pycache__", ".idea", ".vscode":`；L666 `if err := walkRootDir(root, child, b, n, depth+1); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function walkRootDir（internal/control/refs.go:635-672）
- **可信生产调用方（2）：** `readFileRef`（`internal/control/refs.go:481`，调用点 L509，confidence=0.90，exact-match）；`walkRootDir`（`internal/control/refs.go:635`，调用点 L666，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `walkRootDir`（`internal/control/refs.go:635`，调用点 L666，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::resolveRefs`（`internal/control/refs.go:432`，调用点 L446，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/refs.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `walkRootDir` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L636、L645、L654、L656、L666 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `walkRootDir` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/refs.go|walkRootDir` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/refs.go:635-672` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `walkRootDir` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/refs.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/refs.go|walkRootDir` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-025 · 圈复杂度 · `*AskTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-025 |
| Baseline key | `complexity\|internal/agent/ask.go\|*AskTool.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/agent/ask.go` |
| 代码行 | 71 |
| 函数/方法/包 | `*AskTool.Execute` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*AskTool.Execute` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 71-130 行；可见控制流热点：L83 `if err := json.Unmarshal(args, &p); err != nil {`；L91 `for i, q := range p.Questions {`；L93 `if question == "" || len(q.Options) < 2 {`；L100 `if label == "" {`；L103 `if prev, ok := seenLabels[label]; ok {`；L126 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 渲染/输出（formatAnswers）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method AskTool::Execute（internal/agent/ask.go:71-130）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `CallContext`（`internal/agent/agent.go:88`，调用点 L118，confidence=0.90，exact-match）；`formatAnswers`（`internal/agent/ask.go:137`，调用点 L129，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Asker::Ask`（`internal/agent/agent.go:53`，调用点 L125，confidence=0.80，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/agent/ask_test.go`
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/ask.go`、`internal/agent/agent.go`、`internal/agent/ask_test.go`、`internal/agent/argserror_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `*AskTool.Execute` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L83、L91、L98、L103、L126 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*AskTool.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/ask.go|*AskTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/ask.go:71-130` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*AskTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/agent/ask_test.go`。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/ask.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/ask.go|*AskTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-046 · 圈复杂度 · `ListCleanupPending`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-046 |
| Baseline key | `complexity\|internal/agent/save.go\|ListCleanupPending` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/agent/save.go` |
| 代码行 | 273 |
| 函数/方法/包 | `ListCleanupPending` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ListCleanupPending` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 273-315 行；可见控制流热点：L275 `if dir == "" {`；L280 `if os.IsNotExist(err) {`；L286 `for _, e := range entries {`；L293 `if err != nil {`；L294 `if os.IsNotExist(err) {`；L300 `if err := json.Unmarshal(b, &meta); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ListCleanupPending（internal/agent/save.go:273-315）
- **可信生产调用方（1）：** `ReconcileCleanupPending`（`internal/agent/save.go:319`，调用点 L320，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `reconcileDesktopCleanupPending`（`desktop/sessions.go:174`，调用点 L175，confidence=0.90，import，待源码复核）；`ReconcileCleanupPending`（`internal/acp/service.go:1514`，调用点 L1515，confidence=0.90，import，待源码复核）；`ReconcileCleanupPending`（`internal/control/controller.go:2077`，调用点 L2078，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`、`internal/agent/cachehit_e2e_test.go`、`internal/agent/cancel_test.go`、`internal/agent/canonical_todo_test.go`、`internal/agent/compact_loop_e2e_test.go`、`internal/agent/compact_test.go`、`internal/agent/complete_step_e2e_test.go`、`internal/agent/coordinator_test.go`
- **需同步检查：** `internal/agent/save.go`、`internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`

#### 待修改项

- [ ] 先冻结 `ListCleanupPending` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L275、L280、L287、L294、L300 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `ListCleanupPending` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/agent/save.go|ListCleanupPending` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/agent/save.go:273-315` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ListCleanupPending` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/agent/argserror_test.go`、`internal/agent/ask_test.go`、`internal/agent/branch_test.go`、`internal/agent/cache_diagnostics_test.go`、`internal/agent/cache_shape_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/agent`。
- [ ] 运行相关包 vet：`go vet ./internal/agent`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/agent/save.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/agent/save.go|ListCleanupPending` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-203 · 圈复杂度 · `*ProviderAutoPlanClassifier.NeedsPlan`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-203 |
| Baseline key | `complexity\|internal/control/auto_plan_classifier.go\|*ProviderAutoPlanClassifier.NeedsPlan` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/control/auto_plan_classifier.go` |
| 代码行 | 42 |
| 函数/方法/包 | `*ProviderAutoPlanClassifier.NeedsPlan` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*ProviderAutoPlanClassifier.NeedsPlan` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 42-85 行；可见控制流热点：L43 `if c == nil || nilutil.IsNil(c.prov) {`；L60 `for chunk := range ch {`；L62 `case provider.ChunkText:`；L64 `case provider.ChunkUsage:`；L70 `if usage != nil && usage.TotalTokens > 0 && !nilutil.IsNil(c.sink) {`；L81 `if out.NeedsPlan == nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method ProviderAutoPlanClassifier::NeedsPlan（internal/control/auto_plan_classifier.go:42-85）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `IsNil`（`internal/nilutil/nil.go:6`，调用点 L43/L70，confidence=0.90，import，2 条边）
- **待源码复核被调候选（1）：** `extractJSONObject`（`internal/control/auto_plan_classifier.go:87`，调用点 L78，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`、`internal/nilutil`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/auto_plan_classifier.go`、`internal/nilutil/nil.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*ProviderAutoPlanClassifier.NeedsPlan` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L43、L60、L64、L70、L81 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `*ProviderAutoPlanClassifier.NeedsPlan` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/auto_plan_classifier.go|*ProviderAutoPlanClassifier.NeedsPlan` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/auto_plan_classifier.go:42-85` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*ProviderAutoPlanClassifier.NeedsPlan` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/auto_plan_classifier.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/auto_plan_classifier.go|*ProviderAutoPlanClassifier.NeedsPlan` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-214 · 圈复杂度 · `*Controller.Rewind`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-214 |
| Baseline key | `complexity\|internal/control/controller.go\|*Controller.Rewind` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 2111 |
| 函数/方法/包 | `*Controller.Rewind` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Controller.Rewind` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 2111-2150 行；可见控制流热点：L2112 `if !c.checkpoints.enabled() || c.executor == nil {`；L2120 `if scope == RewindCode || scope == RewindBoth {`；L2122 `if err != nil {`；L2130 `if scope == RewindConversation || scope == RewindBoth {`；L2131 `if !hasBound {`；L2143 `if err := c.Snapshot(); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（checkpointManager::truncateFrom、Controller::rewindFail）、持久化/I-O（checkpointManager::restoreCode）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::Rewind（internal/control/controller.go:2111-2150）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `checkpointManager::enabled`（`internal/control/checkpoint.go:54`，调用点 L2112，confidence=0.90，exact-match）；`checkpointManager::boundary`（`internal/control/checkpoint.go:77`，调用点 L2118，confidence=0.90，exact-match）；`checkpointManager::restoreCode`（`internal/control/checkpoint.go:97`，调用点 L2121，confidence=0.90，exact-match）；`checkpointManager::truncateFrom`（`internal/control/checkpoint.go:120`，调用点 L2142，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Controller::rewindFail`（`internal/control/controller.go:2100`，调用点 L2113/L2116/L2123/L2132，confidence=0.70，instance-method，5 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 15 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/control/checkpoint.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.Rewind` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L2112、L2120、L2125、L2131、L2143 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Controller.Rewind` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/controller.go|*Controller.Rewind` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:2111-2150` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.Rewind` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/controller.go|*Controller.Rewind` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-211 · 圈复杂度 · `*Controller.DeleteUserExpert`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-211 |
| Baseline key | `complexity\|internal/control/controller.go\|*Controller.DeleteUserExpert` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 1706 |
| 函数/方法/包 | `*Controller.DeleteUserExpert` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Controller.DeleteUserExpert` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 1706-1743 行；可见控制流热点：L1707 `if c == nil || strings.TrimSpace(c.userExpertPath) == "" {`；L1714 `if c.CurrentExpert().SelectedID == id {`；L1720 `if err != nil {`；L1725 `for _, record := range records {`；L1732 `if !found {`；L1738 `if err := expert.RemoveExpertLibraryID(c.userExpertPath, id); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（LoadUserExperts、Controller::reloadExpertCatalog）、状态与控制（Controller::reloadExpertCatalog）、持久化/I-O（SaveUserExperts）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::DeleteUserExpert（internal/control/controller.go:1706-1743）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `LoadUserExperts`（`internal/expert/user_store.go:121`，调用点 L1719，confidence=0.90，import）；`RemoveExpertLibraryID`（`internal/expert/user_store.go:194`，调用点 L1738，confidence=0.90，import）；`SaveUserExperts`（`internal/expert/user_store.go:238`，调用点 L1735，confidence=0.90，import）
- **待源码复核被调候选（1）：** `Controller::reloadExpertCatalog`（`internal/control/controller.go:1898`，调用点 L1741，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 8 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`、`internal/expert`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/expert/user_store.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.DeleteUserExpert` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1707、L1714、L1725、L1732、L1738 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Controller.DeleteUserExpert` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/controller.go|*Controller.DeleteUserExpert` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:1706-1743` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.DeleteUserExpert` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/controller.go|*Controller.DeleteUserExpert` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-216 · 圈复杂度 · `resolveBranch`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-216 |
| Baseline key | `complexity\|internal/control/controller.go\|resolveBranch` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 2362 |
| 函数/方法/包 | `resolveBranch` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`resolveBranch` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 2362-2387 行；可见控制流热点：L2365 `for _, b := range branches {`；L2368 `case b.ID == ref || strings.EqualFold(b.ID, ref):`；L2370 `case b.Name != "" && nameLower == refLower:`；L2374 `case strings.HasPrefix(strings.ToLower(shortBranchID(b.ID)), refLower):`；L2376 `case b.Path == ref:`；L2383 `if len(matches) > 1 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function resolveBranch（internal/control/controller.go:2362-2387）
- **可信生产调用方（1）：** `Controller::SwitchBranch`（`internal/control/controller.go:2320`，调用点 L2335，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `shortBranchID`（`internal/control/branches.go:143`，调用点 L2374，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::SwitchBranch`（`internal/control/controller.go:2320`，调用点 L2335，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 15 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/control/branches.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `resolveBranch` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L2365、L2368、L2372、L2376、L2383 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `resolveBranch` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/controller.go|resolveBranch` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:2362-2387` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `resolveBranch` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/controller.go|resolveBranch` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-217 · 圈复杂度 · `*Controller.maybeColdResumePrune`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-217 |
| Baseline key | `complexity\|internal/control/controller.go\|*Controller.maybeColdResumePrune` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/control/controller.go` |
| 代码行 | 2538 |
| 函数/方法/包 | `*Controller.maybeColdResumePrune` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Controller.maybeColdResumePrune` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 2538-2563 行；可见控制流热点：L2539 `if c.disableColdResumePrune || c.executor == nil || path == "" {`；L2546 `if err != nil || !ok || m.UpdatedAt.IsZero() {`；L2550 `if time.Since(last) < cacheColdAfter {`；L2554 `if err != nil || st.Results == 0 {`；L2560 `if err := c.Snapshot(); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（LoadBranchMeta）、状态与控制（Agent::PruneStaleToolResults）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Controller::maybeColdResumePrune（internal/control/controller.go:2538-2563）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Controller::Resume`（`internal/control/controller.go:2442`，调用点 L2455，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `LoadBranchMeta`（`internal/agent/branch.go:91`，调用点 L2545，confidence=0.90，import）；`Agent::PruneStaleToolResults`（`internal/agent/prune.go:29`，调用点 L2553，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::Resume`（`internal/control/controller.go:2442`，调用点 L2455，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 7 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/control`、`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`、`internal/control/branches_test.go`、`internal/control/controller_test.go`、`internal/control/errmsg_test.go`、`internal/control/goal_concurrency_test.go`、`internal/control/goal_test.go`、`internal/control/imagecompress_test.go`、`internal/control/input_test.go`
- **需同步检查：** `internal/control/controller.go`、`internal/agent/branch.go`、`internal/agent/prune.go`、`internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`

#### 待修改项

- [ ] 先冻结 `*Controller.maybeColdResumePrune` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L2539、L2546、L2550、L2554、L2560 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Controller.maybeColdResumePrune` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/control/controller.go|*Controller.maybeColdResumePrune` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/control/controller.go:2538-2563` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Controller.maybeColdResumePrune` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/control/approval_e2e_test.go`、`internal/control/attachments_test.go`、`internal/control/auto_plan_classifier_test.go`、`internal/control/auto_plan_e2e_test.go`、`internal/control/auto_plan_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/control`。
- [ ] 运行相关包 vet：`go vet ./internal/control`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/control/controller.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/control/controller.go|*Controller.maybeColdResumePrune` 下降或消失且无新增/恶化。
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
