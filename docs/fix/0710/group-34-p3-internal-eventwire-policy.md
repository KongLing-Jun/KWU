# G34 · internal/eventwire · policy 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G34 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/eventwire`、`internal/planmode`、`internal/acp` |
| 涉及文件 | `internal/eventwire/wire.go`<br>`internal/planmode/policy.go`<br>`internal/acp/service.go` |
| 主要治理目标 | 围绕 `internal/acp/service.go` 等治理对象降低复杂度/体量，统一处理 2 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | eventwire / planmode / acp 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614`、`origin/expert-202607081622`；单文件最多集中 10 条问题 |
| 建议处理顺序 | 总序第 34 组；先于 G41、G48、G49 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 协议、序列化或流式事件兼容；会话、状态或恢复语义；并发、取消或事件顺序；返回值、错误语义和现有调用顺序；权限、安全或凭据边界；持久化、迁移或文件格式兼容；配置、平台或向后兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **43/100**，对象 `ToWire` 的分项为严重度 9.7、集中度 9.6、影响面 11.7、可执行性 12.0；最高超限 2.10x，文件内 3 条问题，CodeGraph 记录 2 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数，必须合并治理。

### 治理边界与组间关系

本组涉及 3 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G41、G48、G49，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-248` | `complexity\|internal/eventwire/wire.go\|ToWire` | 圈复杂度 | `internal/eventwire/wire.go` | `ToWire` | 32 | 21/10 | +11（2.10x） | P3 | 2 可信 + 0 待核 caller / 7 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `FN-035` | `func_lines\|internal/eventwire/wire.go\|ToWire` | 函数有效行数 | `internal/eventwire/wire.go` | `ToWire` | 32 | 87/80 | +7（1.09x） | P3 | 2 可信 + 0 待核 caller / 7 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-323` | `complexity\|internal/planmode/policy.go\|unsafeSafeCommandArg` | 圈复杂度 | `internal/planmode/policy.go` | `unsafeSafeCommandArg` | 365 | 24/10 | +14（2.40x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-011` | `complexity\|internal/acp/service.go\|*service.openExistingSession` | 圈复杂度 | `internal/acp/service.go` | `*service.openExistingSession` | 398 | 22/10 | +12（2.20x） | P3 | 0 可信 + 2 待核 caller / 9 可信 + 13 待核 callee / 0 unresolved | 待修改 |
| `FN-001` | `func_lines\|internal/acp/service.go\|*service.openExistingSession` | 函数有效行数 | `internal/acp/service.go` | `*service.openExistingSession` | 398 | 107/80 | +27（1.34x） | P3 | 0 可信 + 2 待核 caller / 9 可信 + 13 待核 callee / 0 unresolved | 待修改 |
| `CX-324` | `complexity\|internal/planmode/policy.go\|shellFields` | 圈复杂度 | `internal/planmode/policy.go` | `shellFields` | 410 | 16/10 | +6（1.60x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `FL-001` | `file_lines\|internal/acp/service.go\|` | 文件有效行数 | `internal/acp/service.go` | `internal/acp/service.go` | 1 | 1390/800 | +590（1.74x） | P3 | 1 可信 + 0 待核 caller / 17 可信 + 14 待核 callee / 0 unresolved | 待修改 |
| `CX-322` | `complexity\|internal/planmode/policy.go\|decideBash` | 圈复杂度 | `internal/planmode/policy.go` | `decideBash` | 292 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-321` | `complexity\|internal/planmode/policy.go\|Policy.Decide` | 圈复杂度 | `internal/planmode/policy.go` | `Policy.Decide` | 151 | 12/10 | +2（1.20x） | P3 | 0 可信 + 0 待核 caller / 3 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-249` | `complexity\|internal/eventwire/wire.go\|ToWireDecisionAudit` | 圈复杂度 | `internal/eventwire/wire.go` | `ToWireDecisionAudit` | 335 | 12/10 | +2（1.20x） | P3 | 2 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-248 · 圈复杂度 · `ToWire`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-248 |
| Baseline key | `complexity\|internal/eventwire/wire.go\|ToWire` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P1，治理对象得分 43） |
| 文件 | `internal/eventwire/wire.go` |
| 代码行 | 32 |
| 函数/方法/包 | `ToWire` |
| Baseline / 当前复测 / 阈值 | 21 / 21 / 10 |
| 超出 | +11，2.10x |
| 同一治理对象关联问题 | `FN-035` |

#### 明确的问题描述

`ToWire` 的圈复杂度当前为 **21**，超过限制 **10**（+11，2.10x）。定义范围为 32-120 行；可见控制流热点：L34 `if len(e.MemoryCitations) > 0 {`；L44 `case event.ToolDispatch, event.ToolResult, event.ToolProgress:`；L66 `if e.CacheDiagnostics != nil {`；L95 `case event.ApprovalRequest:`；L106 `case event.GuardianAssessment:`；L115 `case event.Retrying:`。CodeGraph 记录的下游职责候选主要涉及 网络/协议（ToWireDecisionAudit、ToWireMemoryCitations、ToWireGuardian、ToWireAsk）、权限/安全（ToWireGuardian）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ToWire（internal/eventwire/wire.go:32-120）
- **可信生产调用方（2）：** `toWireTab`（`desktop/tabs.go:1030`，调用点 L1031，confidence=0.90，import）；`Broadcaster::Emit`（`internal/serve/broadcaster.go:29`，调用点 L30，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（7）：** `ToWireDecisionAudit`（`internal/eventwire/wire.go:335`，调用点 L96/L100，confidence=0.90，exact-match，2 条边）；`ToWireMemoryCitations`（`internal/eventwire/wire.go:151`，调用点 L35，confidence=0.90，exact-match）；`ToWireGuardian`（`internal/eventwire/wire.go:280`，调用点 L107，confidence=0.90，exact-match）；`ToWireAsk`（`internal/eventwire/wire.go:308`，调用点 L98，confidence=0.90，exact-match）；`ToWireCacheDiagnostics`（`internal/eventwire/wire.go:359`，调用点 L67，confidence=0.90，exact-match）；`Pricing::Cost`（`internal/provider/provider.go:481`，调用点 L70，confidence=0.90，exact-match）；`Pricing::Symbol`（`internal/provider/provider.go:498`，调用点 L72，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `tabEventSink::Emit`（`desktop/tabs.go:668`，调用点 L686，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/eventwire`、`desktop`、`internal/serve`、`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/eventwire/wire_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/eventwire/wire.go`、`desktop/tabs.go`、`internal/serve/broadcaster.go`、`internal/eventwire/wire_test.go`

#### 待修改项

- [ ] 先冻结 `ToWire` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L34、L53、L76、L101、L115 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 网络/协议、权限/安全 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `ToWire` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-248、FN-035 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/eventwire/wire.go|ToWire` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/eventwire/wire.go:32-120` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ToWire` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41、G48、G49 处理。
- 重点保护：协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-248`、`FN-035` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/eventwire/wire_test.go`。
- [ ] 运行相关包测试：`go test ./internal/eventwire`。
- [ ] 运行相关包 vet：`go vet ./internal/eventwire`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/eventwire/wire.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/eventwire/wire.go|ToWire` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-035 · 函数有效行数 · `ToWire`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-035 |
| Baseline key | `func_lines\|internal/eventwire/wire.go\|ToWire` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 43） |
| 文件 | `internal/eventwire/wire.go` |
| 代码行 | 32 |
| 函数/方法/包 | `ToWire` |
| Baseline / 当前复测 / 阈值 | 87 / 87 / 80 |
| 超出 | +7，1.09x |
| 同一治理对象关联问题 | `CX-248` |

#### 明确的问题描述

`ToWire` 的有效代码行 **87**，超过限制 **80**（+7）。CodeGraph 定义物理范围为 32-120 行，并记录 7 个直接下游候选；候选职责分布为 网络/协议（ToWireDecisionAudit、ToWireMemoryCitations、ToWireGuardian、ToWireAsk）、权限/安全（ToWireGuardian）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ToWire（internal/eventwire/wire.go:32-120）
- **可信生产调用方（2）：** `toWireTab`（`desktop/tabs.go:1030`，调用点 L1031，confidence=0.90，import）；`Broadcaster::Emit`（`internal/serve/broadcaster.go:29`，调用点 L30，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（7）：** `ToWireDecisionAudit`（`internal/eventwire/wire.go:335`，调用点 L96/L100，confidence=0.90，exact-match，2 条边）；`ToWireMemoryCitations`（`internal/eventwire/wire.go:151`，调用点 L35，confidence=0.90，exact-match）；`ToWireGuardian`（`internal/eventwire/wire.go:280`，调用点 L107，confidence=0.90，exact-match）；`ToWireAsk`（`internal/eventwire/wire.go:308`，调用点 L98，confidence=0.90，exact-match）；`ToWireCacheDiagnostics`（`internal/eventwire/wire.go:359`，调用点 L67，confidence=0.90，exact-match）；`Pricing::Cost`（`internal/provider/provider.go:481`，调用点 L70，confidence=0.90，exact-match）；`Pricing::Symbol`（`internal/provider/provider.go:498`，调用点 L72，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `tabEventSink::Emit`（`desktop/tabs.go:668`，调用点 L686，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/eventwire`、`desktop`、`internal/serve`、`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/eventwire/wire_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/eventwire/wire.go`、`desktop/tabs.go`、`internal/serve/broadcaster.go`、`internal/eventwire/wire_test.go`

#### 待修改项

- [ ] 先冻结 `ToWire` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `ToWire` 的 87 个有效行按 网络/协议、权限/安全 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-248、FN-035 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/eventwire/wire.go|ToWire` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/eventwire/wire.go:32-120` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ToWire` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41、G48、G49 处理。
- 重点保护：协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-248`、`FN-035` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/eventwire/wire_test.go`。
- [ ] 运行相关包测试：`go test ./internal/eventwire`。
- [ ] 运行相关包 vet：`go vet ./internal/eventwire`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/eventwire/wire.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/eventwire/wire.go|ToWire` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-323 · 圈复杂度 · `unsafeSafeCommandArg`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-323 |
| Baseline key | `complexity\|internal/planmode/policy.go\|unsafeSafeCommandArg` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P1，治理对象得分 43） |
| 文件 | `internal/planmode/policy.go` |
| 代码行 | 365 |
| 函数/方法/包 | `unsafeSafeCommandArg` |
| Baseline / 当前复测 / 阈值 | 24 / 24 / 10 |
| 超出 | +14，2.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`unsafeSafeCommandArg` 的圈复杂度当前为 **24**，超过限制 **10**（+14，2.40x）。定义范围为 365-408 行；可见控制流热点：L367 `if err != "" {`；L379 `if strings.HasPrefix(safe, "git ") {`；L386 `switch safe {`；L390 `if arg == "-O" || strings.HasPrefix(arg, "-O") || strings.HasPrefix(lowerArg, "--open-files-in-page…`；L396 `if findWriteArgs[arg] {`；L402 `if goWriteOrExecArgs[arg] || strings.HasPrefix(arg, "-mod=mod") || strings.HasPrefix(arg, "-modfile…`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function unsafeSafeCommandArg（internal/planmode/policy.go:365-408）
- **可信生产调用方（1）：** `decideBash`（`internal/planmode/policy.go:292`，调用点 L332，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `shellFields`（`internal/planmode/policy.go:410`，调用点 L366，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Policy::Decide`（`internal/planmode/policy.go:151`，调用点 L154，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/planmode`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/planmode/marker_test.go`、`internal/planmode/policy_test.go`、`internal/planmode/reconcile_test.go`
- **需同步检查：** `internal/planmode/policy.go`、`internal/planmode/marker_test.go`、`internal/planmode/policy_test.go`、`internal/planmode/reconcile_test.go`

#### 待修改项

- [ ] 逐段核对 L367、L380、L388、L395、L402 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `unsafeSafeCommandArg` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/planmode/policy.go|unsafeSafeCommandArg` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/planmode/policy.go:365-408` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `unsafeSafeCommandArg` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41、G48、G49 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/planmode/marker_test.go`、`internal/planmode/policy_test.go`、`internal/planmode/reconcile_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/planmode`。
- [ ] 运行相关包 race：`go test -race ./internal/planmode`。
- [ ] 运行相关包 vet：`go vet ./internal/planmode`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/planmode/policy.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/planmode/policy.go|unsafeSafeCommandArg` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-011 · 圈复杂度 · `*service.openExistingSession`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-011 |
| Baseline key | `complexity\|internal/acp/service.go\|*service.openExistingSession` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P1，治理对象得分 42） |
| 文件 | `internal/acp/service.go` |
| 代码行 | 398 |
| 函数/方法/包 | `*service.openExistingSession` |
| Baseline / 当前复测 / 阈值 | 22 / 22 / 10 |
| 超出 | +12，2.20x |
| 同一治理对象关联问题 | `FN-001` |

#### 明确的问题描述

`*service.openExistingSession` 的圈复杂度当前为 **22**，超过限制 **10**（+12，2.20x）。定义范围为 398-512 行；可见控制流热点：L399 `if err := validateSessionID(method, id); err != nil {`；L411 `if sess := s.session(id); sess != nil {`；L427 `if dir := s.sessionDir(); dir != "" {`；L444 `if err != nil && (strings.TrimSpace(saved.Model) != "" || saved.EffortOverride != nil) {`；L472 `if path != persistedPath && agent.IsCleanupPending(path) {`；L508 `if replay {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（metadataForLoadedSession、loadACPMeta、validateSessionID、LoadSession）、状态与控制（newUpdateSink、updateSink::replay、service::sessionConfigState、updateSink::bindApprove）、持久化/I-O（saveACPMeta）、权限/安全（updateSink::bindApprove）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method service::openExistingSession（internal/acp/service.go:398-512）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `service::sessionLoad`（`internal/acp/service.go:372`，调用点 L377，confidence=0.70，instance-method，待源码复核）；`service::sessionResume`（`internal/acp/service.go:386`，调用点 L391，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（9）：** `IsCleanupPending`（`internal/agent/save.go:256`，调用点 L412/L429/L472，confidence=0.90，import，3 条边）；`newUpdateSink`（`internal/acp/dispatch.go:55`，调用点 L416/L451，confidence=0.90，exact-match，2 条边）；`clonePluginSpecs`（`internal/acp/service.go:1045`，调用点 L492，confidence=0.90，exact-match）；`metadataForLoadedSession`（`internal/acp/service.go:1269`，调用点 L483，confidence=0.90，exact-match）；`loadACPMeta`（`internal/acp/service.go:1303`，调用点 L432，confidence=0.90，exact-match）；`saveACPMeta`（`internal/acp/service.go:1322`，调用点 L499，confidence=0.90，exact-match）；`validateSessionID`（`internal/acp/service.go:1445`，调用点 L399，confidence=0.90，exact-match）；`mcpSpecs`（`internal/acp/service.go:1537`，调用点 L406，confidence=0.90，exact-match）
- **待源码复核被调候选（13）：** `cloneStringPtr`（`internal/acp/service.go:1022`，调用点 L441/L457/L485/L494，confidence=0.70，exact-match，4 条边，待源码复核）；`updateSink::replay`（`internal/acp/dispatch.go:186`，调用点 L416/L509，confidence=0.85，instance-method，2 条边，待源码复核）；`transcriptPath`（`internal/acp/service.go:517`，调用点 L428/L471，confidence=0.70，exact-match，2 条边，待源码复核）；`service::sessionConfigState`（`internal/acp/service.go:957`，调用点 L443/L445，confidence=0.70，instance-method，2 条边，待源码复核）；`updateSink::bindApprove`（`internal/acp/dispatch.go:61`，调用点 L463，confidence=0.70，instance-method，待源码复核）；`updateSink::bindAnswer`（`internal/acp/dispatch.go:71`，调用点 L464，confidence=0.70，instance-method，待源码复核）；`Factory::NewSession`（`internal/acp/service.go:59`，调用点 L452，confidence=0.70，exact-match，待源码复核）；`service::session`（`internal/acp/service.go:917`，调用点 L411，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `service::sessionLoad`（`internal/acp/service.go:372`，调用点 L377，confidence=0.70，instance-method，待源码复核）；`service::sessionResume`（`internal/acp/service.go:386`，调用点 L391，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/acp`、`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`、`internal/acp/server_test.go`
- **需同步检查：** `internal/acp/service.go`、`internal/agent/save.go`、`internal/acp/dispatch.go`、`internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`

#### 待修改项

- [ ] 先冻结 `*service.openExistingSession` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L399、L412、L433、L467、L508 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*service.openExistingSession` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-011、FN-001 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/acp/service.go|*service.openExistingSession` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/acp/service.go:398-512` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*service.openExistingSession` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41、G48、G49 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-011`、`FN-001` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/acp`。
- [ ] 运行相关包 vet：`go vet ./internal/acp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/acp/service.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/acp/service.go|*service.openExistingSession` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-001 · 函数有效行数 · `*service.openExistingSession`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-001 |
| Baseline key | `func_lines\|internal/acp/service.go\|*service.openExistingSession` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/acp/service.go` |
| 代码行 | 398 |
| 函数/方法/包 | `*service.openExistingSession` |
| Baseline / 当前复测 / 阈值 | 107 / 107 / 80 |
| 超出 | +27，1.34x |
| 同一治理对象关联问题 | `CX-011` |

#### 明确的问题描述

`*service.openExistingSession` 的有效代码行 **107**，超过限制 **80**（+27）。CodeGraph 定义物理范围为 398-512 行，并记录 22 个直接下游候选；候选职责分布为 配置/解析（metadataForLoadedSession、loadACPMeta、validateSessionID、LoadSession）、状态与控制（newUpdateSink、updateSink::replay、service::sessionConfigState、updateSink::bindApprove）、持久化/I-O（saveACPMeta）、权限/安全（updateSink::bindApprove）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method service::openExistingSession（internal/acp/service.go:398-512）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `service::sessionLoad`（`internal/acp/service.go:372`，调用点 L377，confidence=0.70，instance-method，待源码复核）；`service::sessionResume`（`internal/acp/service.go:386`，调用点 L391，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（9）：** `IsCleanupPending`（`internal/agent/save.go:256`，调用点 L412/L429/L472，confidence=0.90，import，3 条边）；`newUpdateSink`（`internal/acp/dispatch.go:55`，调用点 L416/L451，confidence=0.90，exact-match，2 条边）；`clonePluginSpecs`（`internal/acp/service.go:1045`，调用点 L492，confidence=0.90，exact-match）；`metadataForLoadedSession`（`internal/acp/service.go:1269`，调用点 L483，confidence=0.90，exact-match）；`loadACPMeta`（`internal/acp/service.go:1303`，调用点 L432，confidence=0.90，exact-match）；`saveACPMeta`（`internal/acp/service.go:1322`，调用点 L499，confidence=0.90，exact-match）；`validateSessionID`（`internal/acp/service.go:1445`，调用点 L399，confidence=0.90，exact-match）；`mcpSpecs`（`internal/acp/service.go:1537`，调用点 L406，confidence=0.90，exact-match）
- **待源码复核被调候选（13）：** `cloneStringPtr`（`internal/acp/service.go:1022`，调用点 L441/L457/L485/L494，confidence=0.70，exact-match，4 条边，待源码复核）；`updateSink::replay`（`internal/acp/dispatch.go:186`，调用点 L416/L509，confidence=0.85，instance-method，2 条边，待源码复核）；`transcriptPath`（`internal/acp/service.go:517`，调用点 L428/L471，confidence=0.70，exact-match，2 条边，待源码复核）；`service::sessionConfigState`（`internal/acp/service.go:957`，调用点 L443/L445，confidence=0.70，instance-method，2 条边，待源码复核）；`updateSink::bindApprove`（`internal/acp/dispatch.go:61`，调用点 L463，confidence=0.70，instance-method，待源码复核）；`updateSink::bindAnswer`（`internal/acp/dispatch.go:71`，调用点 L464，confidence=0.70，instance-method，待源码复核）；`Factory::NewSession`（`internal/acp/service.go:59`，调用点 L452，confidence=0.70，exact-match，待源码复核）；`service::session`（`internal/acp/service.go:917`，调用点 L411，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `service::sessionLoad`（`internal/acp/service.go:372`，调用点 L377，confidence=0.70，instance-method，待源码复核）；`service::sessionResume`（`internal/acp/service.go:386`，调用点 L391，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/acp`、`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`、`internal/acp/server_test.go`
- **需同步检查：** `internal/acp/service.go`、`internal/agent/save.go`、`internal/acp/dispatch.go`、`internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`

#### 待修改项

- [ ] 先冻结 `*service.openExistingSession` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*service.openExistingSession` 的 107 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-011、FN-001 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/acp/service.go|*service.openExistingSession` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/acp/service.go:398-512` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*service.openExistingSession` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41、G48、G49 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-011`、`FN-001` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/acp`。
- [ ] 运行相关包 vet：`go vet ./internal/acp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/acp/service.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/acp/service.go|*service.openExistingSession` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-324 · 圈复杂度 · `shellFields`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-324 |
| Baseline key | `complexity\|internal/planmode/policy.go\|shellFields` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 38） |
| 文件 | `internal/planmode/policy.go` |
| 代码行 | 410 |
| 函数/方法/包 | `shellFields` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`shellFields` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 410-480 行；可见控制流热点：L418 `if haveField {`；L431 `if inSingle {`；L441 `switch r {`；L453 `case unicode.IsSpace(r):`；L461 `case r == '\\':`；L475 `if inDouble {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function shellFields（internal/planmode/policy.go:410-480）
- **可信生产调用方（1）：** `unsafeSafeCommandArg`（`internal/planmode/policy.go:365`，调用点 L366，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/planmode`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/planmode/marker_test.go`、`internal/planmode/policy_test.go`、`internal/planmode/reconcile_test.go`
- **需同步检查：** `internal/planmode/policy.go`、`internal/planmode/marker_test.go`、`internal/planmode/policy_test.go`、`internal/planmode/reconcile_test.go`

#### 待修改项

- [ ] 逐段核对 L418、L432、L444、L458、L475 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `shellFields` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/planmode/policy.go|shellFields` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/planmode/policy.go:410-480` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `shellFields` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41、G48、G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/planmode/marker_test.go`、`internal/planmode/policy_test.go`、`internal/planmode/reconcile_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/planmode`。
- [ ] 运行相关包 vet：`go vet ./internal/planmode`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/planmode/policy.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/planmode/policy.go|shellFields` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FL-001 · 文件有效行数 · `internal/acp/service.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-001 |
| Baseline key | `file_lines\|internal/acp/service.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 37） |
| 文件 | `internal/acp/service.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/acp/service.go` |
| Baseline / 当前复测 / 阈值 | 1390 / 1390 / 800 |
| 超出 | +590，1.74x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/acp/service.go` 当前有效代码行 **1390**，超过限制 **800**（+590）。CodeGraph 在文件中索引到的主要大符号包括 service::openExistingSession@398-512、service::rebuildSession@667-743、service::sessionNew@294-365、availableCommandsFor@1165-1226、service::sessionPrompt@525-577、service::sessionList@810-858；文件同时承载的调用影响覆盖 16 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 76 个函数/方法节点。
- **可信生产调用方（1）：** `acpCommand`（`internal/cli/acp.go:34`，调用点 L46，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（17）：** `IsCleanupPending`（`internal/agent/save.go:256`，调用点 L412/L429/L472/L1377，confidence=0.90，import，4 条边）；`newUpdateSink`（`internal/acp/dispatch.go:55`，调用点 L416/L451/L319，confidence=0.90，exact-match，3 条边）；`updateSink::setTurnContext`（`internal/acp/dispatch.go:75`，调用点 L544，confidence=0.90，exact-match）；`updateSink::clearTurnContext`（`internal/acp/dispatch.go:81`，调用点 L546，confidence=0.90，exact-match）；`FlattenPrompt`（`internal/acp/protocol.go:337`，调用点 L534，confidence=0.90，exact-match）；`NewConn`（`internal/acp/server.go:100`，调用点 L106，confidence=0.90，exact-match）；`BranchID`（`internal/agent/branch.go:76`，调用点 L1503，confidence=0.90，import）；`LoadSession`（`internal/agent/save.go:62`，调用点 L476，confidence=0.90，import）
- **待源码复核被调候选（14）：** `Conn::Handle`（`internal/acp/server.go:115`，调用点 L113/L114/L115/L116，confidence=0.80，instance-method，11 条边，待源码复核）；`updateSink::bindApprove`（`internal/acp/dispatch.go:61`，调用点 L463/L721/L332，confidence=0.70，instance-method，3 条边，待源码复核）；`updateSink::bindAnswer`（`internal/acp/dispatch.go:71`，调用点 L464/L722/L333，confidence=0.70，instance-method，3 条边，待源码复核）；`updateSink::send`（`internal/acp/dispatch.go:178`，调用点 L686/L552/L1159，confidence=0.70，exact-match，3 条边，待源码复核）；`updateSink::replay`（`internal/acp/dispatch.go:186`，调用点 L416/L509，confidence=0.85，instance-method，2 条边，待源码复核）；`Turn::Finish`（`internal/memorycompiler/runtime.go:1728`，调用点 L899/L1528，confidence=0.70，instance-method，2 条边，待源码复核）；`updateSink::Emit`（`internal/acp/dispatch.go:99`，调用点 L549，confidence=0.70，exact-match，待源码复核）；`Conn::HandleNotify`（`internal/acp/server.go:118`，调用点 L124，confidence=0.80，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 16 个文件；`internal/acp`、`internal/cli`、`internal/agent`、`internal/memorycompiler`、`internal/automation`、`internal/control`、`internal/fileutil`、`internal/jobs`、`internal/plugin`、`internal/store`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/acp/server_test.go`、`internal/acp/e2e_test.go`、`internal/acp/protocol_test.go`
- **同目录测试候选：** `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/live_test.go`
- **需同步检查：** `internal/acp/service.go`、`internal/cli/acp.go`、`internal/acp/server.go`、`internal/agent/save.go`、`internal/acp/dispatch.go`、`internal/acp/server_test.go`、`internal/acp/e2e_test.go`、`internal/acp/protocol_test.go`、`internal/acp/dispatch_partial_test.go`

#### 待修改项

- [ ] 先冻结 `internal/acp/service.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `service::openExistingSession`、`service::rebuildSession`、`service::sessionNew`、`availableCommandsFor`、`service::sessionPrompt` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-011、CX-012、CX-013、CX-014、CX-015、CX-016、CX-017、CX-018、FN-001。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/acp/service.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/acp/service.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/acp/service.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41、G48、G49 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/acp/server_test.go`、`internal/acp/e2e_test.go`、`internal/acp/protocol_test.go`。
- [ ] 运行相关包测试：`go test ./internal/acp`。
- [ ] 运行相关包 race：`go test -race ./internal/acp`。
- [ ] 运行相关包 vet：`go vet ./internal/acp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/acp/service.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/acp/service.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-322 · 圈复杂度 · `decideBash`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-322 |
| Baseline key | `complexity\|internal/planmode/policy.go\|decideBash` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/planmode/policy.go` |
| 代码行 | 292 |
| 函数/方法/包 | `decideBash` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`decideBash` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 292-352 行；可见控制流热点：L298 `if err := json.Unmarshal(args, &p); err != nil || strings.TrimSpace(p.Command) == "" {`；L310 `if p.PreserveBackgroundProcesses {`；L319 `for _, mc := range bashMetachars {`；L328 `for _, safe := range safeBashCommands {`；L329 `if !bashMatchesSafePrefix(lower, safe) {`；L339 `if arg != "" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function decideBash（internal/planmode/policy.go:292-352）
- **可信生产调用方（1）：** `Policy::Decide`（`internal/planmode/policy.go:151`，调用点 L154，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `bashMatchesSafePrefix`（`internal/planmode/policy.go:354`，调用点 L329，confidence=0.90，exact-match）；`unsafeSafeCommandArg`（`internal/planmode/policy.go:365`，调用点 L332，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Policy::Decide`（`internal/planmode/policy.go:151`，调用点 L154，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/planmode`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/planmode/marker_test.go`、`internal/planmode/policy_test.go`、`internal/planmode/reconcile_test.go`
- **需同步检查：** `internal/planmode/policy.go`、`internal/planmode/marker_test.go`、`internal/planmode/policy_test.go`、`internal/planmode/reconcile_test.go`

#### 待修改项

- [ ] 逐段核对 L298、L310、L320、L329、L339 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `decideBash` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/planmode/policy.go|decideBash` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/planmode/policy.go:292-352` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `decideBash` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41、G48、G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/planmode/marker_test.go`、`internal/planmode/policy_test.go`、`internal/planmode/reconcile_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/planmode`。
- [ ] 运行相关包 vet：`go vet ./internal/planmode`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/planmode/policy.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/planmode/policy.go|decideBash` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-321 · 圈复杂度 · `Policy.Decide`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-321 |
| Baseline key | `complexity\|internal/planmode/policy.go\|Policy.Decide` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/planmode/policy.go` |
| 代码行 | 151 |
| 函数/方法/包 | `Policy.Decide` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Policy.Decide` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 151-190 行；可见控制流热点：L153 `if name == "bash" {`；L159 `if call.Safety == PlanSafetyUnsafe {`；L162 `if alwaysAllowedTools[name] {`；L166 `if !call.ReadOnly {`；L171 `if call.ReadOnly && !call.Untrusted {`；L180 `if call.ReadOnly && call.Untrusted {`。CodeGraph 记录的下游职责候选主要涉及 权限/安全（Policy::allowed）、任务/并发（blockKnown）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Policy::Decide（internal/planmode/policy.go:151-190）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `blockKnown`（`internal/planmode/policy.go:219`，调用点 L157/L160，confidence=0.90，exact-match，2 条边）；`planSafeContractViolation`（`internal/planmode/policy.go:232`，调用点 L167，confidence=0.90，exact-match）；`decideBash`（`internal/planmode/policy.go:292`，调用点 L154，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Policy::allowed`（`internal/planmode/policy.go:210`，调用点 L177，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/planmode`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/planmode/policy_test.go`
- **同目录测试候选：** `internal/planmode/marker_test.go`、`internal/planmode/reconcile_test.go`
- **需同步检查：** `internal/planmode/policy.go`、`internal/planmode/policy_test.go`、`internal/planmode/marker_test.go`、`internal/planmode/reconcile_test.go`

#### 待修改项

- [ ] 逐段核对 L153、L159、L165、L171、L180 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 权限/安全、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Policy.Decide` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/planmode/policy.go|Policy.Decide` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/planmode/policy.go:151-190` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Policy.Decide` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41、G48、G49 处理。
- 重点保护：权限、安全或凭据边界。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/planmode/policy_test.go`。
- [ ] 运行相关包测试：`go test ./internal/planmode`。
- [ ] 运行相关包 vet：`go vet ./internal/planmode`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/planmode/policy.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/planmode/policy.go|Policy.Decide` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-249 · 圈复杂度 · `ToWireDecisionAudit`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-249 |
| Baseline key | `complexity\|internal/eventwire/wire.go\|ToWireDecisionAudit` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 27） |
| 文件 | `internal/eventwire/wire.go` |
| 代码行 | 335 |
| 函数/方法/包 | `ToWireDecisionAudit` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ToWireDecisionAudit` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 335-356 行；可见控制流热点：L336 `if a == nil {`；L339 `if a.ActorID == "" && a.ProfileID == "" && a.SessionID == "" &&`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ToWireDecisionAudit（internal/eventwire/wire.go:335-356）
- **可信生产调用方（2）：** `ToWire`（`internal/eventwire/wire.go:32`，调用点 L96/L100，confidence=0.90，exact-match，2 条边）；`ToWireAsk`（`internal/eventwire/wire.go:308`，调用点 L317，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/eventwire`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/eventwire/wire_test.go`
- **需同步检查：** `internal/eventwire/wire.go`、`internal/eventwire/wire_test.go`

#### 待修改项

- [ ] 先冻结 `ToWireDecisionAudit` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L336、L339 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `ToWireDecisionAudit` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/eventwire/wire.go|ToWireDecisionAudit` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/eventwire/wire.go:335-356` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ToWireDecisionAudit` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G41、G48、G49 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/eventwire/wire_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/eventwire`。
- [ ] 运行相关包 vet：`go vet ./internal/eventwire`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/eventwire/wire.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/eventwire/wire.go|ToWireDecisionAudit` 下降或消失且无新增/恶化。
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
