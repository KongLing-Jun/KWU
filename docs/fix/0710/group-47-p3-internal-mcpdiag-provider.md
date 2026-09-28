# G47 · internal/mcpdiag · provider 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G47 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/mcpdiag`、`internal/permission`、`internal/provider`、`internal/userctx`、`internal/checkpoint`、`internal/evidence` |
| 涉及文件 | `internal/mcpdiag/auth.go`<br>`internal/permission/permission.go`<br>`internal/provider/provider.go`<br>`internal/userctx/context.go`<br>`internal/checkpoint/checkpoint.go`<br>`internal/evidence/evidence.go` |
| 主要治理目标 | 围绕 `internal/evidence/evidence.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | mcpdiag / permission / provider / userctx 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614`、`origin/expert-202607081622`；单文件最多集中 8 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 47 组；等待 G44 的同文件变更合并 |
| 前置依赖 | G44（internal/evidence/evidence.go） |
| 后续验证重点 | 权限、安全或凭据边界；配置、平台或向后兼容；协议、序列化或流式事件兼容；返回值、错误语义和现有调用顺序；会话、状态或恢复语义 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **35/100**，对象 `HasAuthConfig` 的分项为严重度 2.5、集中度 2.3、影响面 10.2、可执行性 20.0；最高超限 1.10x，文件内 1 条问题，CodeGraph 记录 2 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 6 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G44。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-294` | `complexity\|internal/mcpdiag/auth.go\|HasAuthConfig` | 圈复杂度 | `internal/mcpdiag/auth.go` | `HasAuthConfig` | 55 | 11/10 | +1（1.10x） | P3 | 2 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-320` | `complexity\|internal/permission/permission.go\|*Gate.Check` | 圈复杂度 | `internal/permission/permission.go` | `*Gate.Check` | 369 | 13/10 | +3（1.30x） | P3 | 0 可信 + 0 待核 caller / 3 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-345` | `complexity\|internal/provider/provider.go\|closeTruncatedJSON` | 圈复杂度 | `internal/provider/provider.go` | `closeTruncatedJSON` | 270 | 17/10 | +7（1.70x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-394` | `complexity\|internal/userctx/context.go\|ValidateProfileID` | 圈复杂度 | `internal/userctx/context.go` | `ValidateProfileID` | 72 | 16/10 | +6（1.60x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-101` | `complexity\|internal/checkpoint/checkpoint.go\|*Store.RestoreCode` | 圈复杂度 | `internal/checkpoint/checkpoint.go` | `*Store.RestoreCode` | 239 | 14/10 | +4（1.40x） | P3 | 0 可信 + 1 待核 caller / 3 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-251` | `complexity\|internal/evidence/evidence.go\|*Ledger.UnverifiedCompletedTodos` | 圈复杂度 | `internal/evidence/evidence.go` | `*Ledger.UnverifiedCompletedTodos` | 416 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 4 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-254` | `complexity\|internal/evidence/evidence.go\|completeStepHasProof` | 圈复杂度 | `internal/evidence/evidence.go` | `completeStepHasProof` | 718 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-346` | `complexity\|internal/provider/provider.go\|backfillToolCallNames` | 圈复杂度 | `internal/provider/provider.go` | `backfillToolCallNames` | 387 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-250` | `complexity\|internal/evidence/evidence.go\|*Ledger.TouchedPaths` | 圈复杂度 | `internal/evidence/evidence.go` | `*Ledger.TouchedPaths` | 146 | 14/10 | +4（1.40x） | P3 | 0 可信 + 1 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-347` | `complexity\|internal/provider/provider.go\|currencySymbol` | 圈复杂度 | `internal/provider/provider.go` | `currencySymbol` | 505 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-294 · 圈复杂度 · `HasAuthConfig`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-294 |
| Baseline key | `complexity\|internal/mcpdiag/auth.go\|HasAuthConfig` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/mcpdiag/auth.go` |
| 代码行 | 55 |
| 函数/方法/包 | `HasAuthConfig` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`HasAuthConfig` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 55-76 行；可见控制流热点：L56 `for k, v := range headers {`；L57 `if strings.TrimSpace(k) == "" {`；L60 `if strings.TrimSpace(v) != "" && (isAuthish(k) || containsExplicitAuthMaterial(v)) {`；L67 `for k, v := range env {`；L68 `if strings.TrimSpace(v) == "" {`；L71 `if isAuthish(k) || containsAuthMaterial(v) {`。CodeGraph 记录的下游职责候选主要涉及 权限/安全（containsAuthMaterial、isAuthish、containsExplicitAuthMaterial）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function HasAuthConfig（internal/mcpdiag/auth.go:55-76）
- **可信生产调用方（2）：** `withPluginConfig`（`desktop/app.go:4840`，调用点 L4862，confidence=0.90，import）；`withMCPPluginConfig`（`internal/cli/mcp_manager.go:368`，调用点 L380，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `containsAuthMaterial`（`internal/mcpdiag/auth.go:110`，调用点 L64/L71，confidence=0.90，exact-match，2 条边）；`isAuthish`（`internal/mcpdiag/auth.go:126`，调用点 L60/L71，confidence=0.90，exact-match，2 条边）；`containsExplicitAuthMaterial`（`internal/mcpdiag/auth.go:115`，调用点 L60，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::SetMCPServerEnabled`（`desktop/app.go:5517`，调用点 L5549，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/mcpdiag`、`desktop`、`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/mcpdiag/auth_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/mcpdiag/auth.go`、`desktop/app.go`、`internal/cli/mcp_manager.go`、`internal/mcpdiag/auth_test.go`

#### 待修改项

- [ ] 逐段核对 L56、L60、L64、L68、L71 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 权限/安全 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `HasAuthConfig` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/mcpdiag/auth.go|HasAuthConfig` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/mcpdiag/auth.go:55-76` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `HasAuthConfig` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/mcpdiag/auth_test.go`。
- [ ] 运行相关包测试：`go test ./internal/mcpdiag`。
- [ ] 运行相关包 vet：`go vet ./internal/mcpdiag`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/mcpdiag/auth.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/mcpdiag/auth.go|HasAuthConfig` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-320 · 圈复杂度 · `*Gate.Check`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-320 |
| Baseline key | `complexity\|internal/permission/permission.go\|*Gate.Check` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/permission/permission.go` |
| 代码行 | 369 |
| 函数/方法/包 | `*Gate.Check` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Gate.Check` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 369-413 行；可见控制流热点：L370 `if toolName == "bash" && !readOnly {`；L376 `switch g.Policy.Decide(toolName, readOnly, args) {`；L379 `case Ask:`；L385 `if err != nil {`；L390 `if approverReason != "" {`；L405 `if rule, ok := ParseRule(toolName); ok {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（ParseRule）、持久化/I-O（isReadOnlyBashSubject）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Gate::Check（internal/permission/permission.go:369-413）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `Subject`（`internal/permission/permission.go:260`，调用点 L371/L383，confidence=0.90，exact-match，2 条边）；`isReadOnlyBashSubject`（`internal/permission/bash_readonly.go:62`，调用点 L372，confidence=0.90，exact-match）；`ParseRule`（`internal/permission/permission.go:72`，调用点 L405，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Policy::Decide`（`internal/permission/permission.go:134`，调用点 L376，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/permission`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/permission/bash_readonly_test.go`、`internal/permission/permission_extra_test.go`、`internal/permission/permission_test.go`
- **需同步检查：** `internal/permission/permission.go`、`internal/permission/bash_readonly.go`、`internal/permission/bash_readonly_test.go`、`internal/permission/permission_extra_test.go`、`internal/permission/permission_test.go`

#### 待修改项

- [ ] 逐段核对 L370、L377、L380、L390、L405 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Gate.Check` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/permission/permission.go|*Gate.Check` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/permission/permission.go:369-413` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Gate.Check` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/permission/bash_readonly_test.go`、`internal/permission/permission_extra_test.go`、`internal/permission/permission_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/permission`。
- [ ] 运行相关包 vet：`go vet ./internal/permission`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/permission/permission.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/permission/permission.go|*Gate.Check` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-345 · 圈复杂度 · `closeTruncatedJSON`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-345 |
| Baseline key | `complexity\|internal/provider/provider.go\|closeTruncatedJSON` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 35） |
| 文件 | `internal/provider/provider.go` |
| 代码行 | 270 |
| 函数/方法/包 | `closeTruncatedJSON` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`closeTruncatedJSON` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 270-320 行；可见控制流热点：L273 `for i := 0; i < len(s); i++ {`；L279 `case c == '\\':`；L287 `case '"':`；L294 `if len(stack) > 0 {`；L307 `switch {`；L316 `if !json.Valid([]byte(out)) {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function closeTruncatedJSON（internal/provider/provider.go:270-320）
- **可信生产调用方（1）：** `repairToolCallArgs`（`internal/provider/provider.go:244`，调用点 L261，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/image_test.go`、`internal/provider/openai/effort_test.go`、`internal/provider/openai/fetch_models_test.go`、`internal/provider/openai/host_test.go`、`internal/provider/openai/image_test.go`、`internal/provider/openai/openai_test.go`、`internal/provider/openai/realcache_test.go`、`internal/provider/openai/reconnect_test.go`、`internal/provider/openai/stall_test.go`
- **需同步检查：** `internal/provider/provider.go`、`internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/image_test.go`

#### 待修改项

- [ ] 先冻结 `closeTruncatedJSON` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L273、L281、L291、L307、L316 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `closeTruncatedJSON` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/provider/provider.go|closeTruncatedJSON` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/provider/provider.go:270-320` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `closeTruncatedJSON` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/image_test.go`、`internal/provider/openai/effort_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/provider`。
- [ ] 运行相关包 vet：`go vet ./internal/provider`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/provider/provider.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/provider/provider.go|closeTruncatedJSON` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-394 · 圈复杂度 · `ValidateProfileID`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-394 |
| Baseline key | `complexity\|internal/userctx/context.go\|ValidateProfileID` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 35） |
| 文件 | `internal/userctx/context.go` |
| 代码行 | 72 |
| 函数/方法/包 | `ValidateProfileID` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ValidateProfileID` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 72-93 行；可见控制流热点：L73 `if profileID == "" || strings.TrimSpace(profileID) != profileID {`；L79 `if strings.HasPrefix(profileID, ".") || strings.Contains(profileID, "..") {`；L82 `for _, r := range profileID {`；L84 `case r >= 'a' && r <= 'z':`；L85 `case r >= 'A' && r <= 'Z':`；L87 `case r == '_' || r == '-' || r == '.':`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ValidateProfileID（internal/userctx/context.go:72-93）
- **可信生产调用方（1）：** `NormalizeProfileID`（`internal/userctx/context.go:64`，调用点 L66，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Profile::Normalize`（`internal/userctx/context.go:23`，调用点 L24，confidence=0.90，exact-match，待源码复核）；`ActorContext::Normalize`（`internal/userctx/context.go:46`，调用点 L52，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/userctx`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/userctx/context_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/userctx/context.go`、`internal/userctx/context_test.go`

#### 待修改项

- [ ] 逐段核对 L73、L79、L83、L85、L87 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `ValidateProfileID` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/userctx/context.go|ValidateProfileID` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/userctx/context.go:72-93` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ValidateProfileID` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/userctx/context_test.go`。
- [ ] 运行相关包测试：`go test ./internal/userctx`。
- [ ] 运行相关包 vet：`go vet ./internal/userctx`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/userctx/context.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/userctx/context.go|ValidateProfileID` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-101 · 圈复杂度 · `*Store.RestoreCode`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-101 |
| Baseline key | `complexity\|internal/checkpoint/checkpoint.go\|*Store.RestoreCode` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/checkpoint/checkpoint.go` |
| 代码行 | 239 |
| 函数/方法/包 | `*Store.RestoreCode` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Store.RestoreCode` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 239-291 行；可见控制流热点：L244 `for _, c := range s.all() {`；L248 `for _, f := range c.Files {`；L259 `for _, p := range order {`；L266 `if snap.Content == nil {`；L274 `if mkErr := os.MkdirAll(filepath.Dir(abs), 0o755); mkErr != nil {`；L284 `if wErr := os.WriteFile(abs, fileenc.Encode(*snap.Content, enc), 0o644); wErr != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Encode）、持久化/I-O（Store::all）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::RestoreCode（internal/checkpoint/checkpoint.go:239-291）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `checkpointManager::restoreCode`（`internal/control/checkpoint.go:97`，调用点 L104，confidence=0.80，instance-method，待源码复核）
- **可信被调用函数（3）：** `detectCurrentEncoding`（`internal/checkpoint/checkpoint.go:293`，调用点 L281，confidence=0.90，exact-match）；`safePath`（`internal/checkpoint/checkpoint.go:305`，调用点 L260，confidence=0.90，exact-match）；`Encode`（`internal/fileutil/encoding/encoding.go:166`，调用点 L284，confidence=0.90，import）
- **待源码复核被调候选（1）：** `Store::all`（`internal/checkpoint/checkpoint.go:226`，调用点 L244，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Controller::Rewind`（`internal/control/controller.go:2111`，调用点 L2121，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/checkpoint`、`internal/control`、`internal/fileutil`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/checkpoint/checkpoint_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/checkpoint/checkpoint.go`、`internal/control/checkpoint.go`、`internal/fileutil/encoding/encoding.go`、`internal/checkpoint/checkpoint_test.go`

#### 待修改项

- [ ] 逐段核对 L244、L249、L261、L274、L284 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Store.RestoreCode` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/checkpoint/checkpoint.go|*Store.RestoreCode` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/checkpoint/checkpoint.go:239-291` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Store.RestoreCode` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/checkpoint/checkpoint_test.go`。
- [ ] 运行相关包测试：`go test ./internal/checkpoint`。
- [ ] 运行相关包 vet：`go vet ./internal/checkpoint`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/checkpoint/checkpoint.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/checkpoint/checkpoint.go|*Store.RestoreCode` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-251 · 圈复杂度 · `*Ledger.UnverifiedCompletedTodos`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-251 |
| Baseline key | `complexity\|internal/evidence/evidence.go\|*Ledger.UnverifiedCompletedTodos` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/evidence/evidence.go` |
| 代码行 | 416 |
| 函数/方法/包 | `*Ledger.UnverifiedCompletedTodos` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Ledger.UnverifiedCompletedTodos` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 416-465 行；可见控制流热点：L418 `if l == nil {`；L430 `if !r.Success || r.ToolName != "todo_write" {`；L438 `if !hasBaseline {`；L443 `if todoStatus(t.Status) != "completed" {`；L447 `if previousTodoCompleted(index, t, previous) {`；L453 `if hasFailedCompleteStepRecoveryForTodo(receipts, baseline, index, current) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeTodos）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Ledger::UnverifiedCompletedTodos（internal/evidence/evidence.go:416-465）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `verifyTodoCompletionTransitions`（`internal/tool/builtin/todo.go:95`，调用点 L100，confidence=0.80，instance-method，待源码复核）
- **可信被调用函数（4）：** `hasFailedCompleteStepRecoveryForTodo`（`internal/evidence/evidence.go:467`，调用点 L453，confidence=0.90，exact-match）；`normalizeTodos`（`internal/evidence/evidence.go:758`，调用点 L417，confidence=0.90，exact-match）；`previousTodoCompleted`（`internal/evidence/evidence.go:777`，调用点 L447，confidence=0.90，exact-match）；`hasSuccessfulCompleteStepForTodo`（`internal/evidence/evidence.go:796`，调用点 L450，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `todoStatus`（`internal/evidence/evidence.go:769`，调用点 L443/L460，confidence=0.70，exact-match，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** `todoWrite::Execute`（`internal/tool/builtin/todo.go:62`，调用点 L88，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/evidence`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/evidence/evidence_test.go`
- **同目录测试候选：** `internal/evidence/commandmatch_test.go`、`internal/evidence/readiness_test.go`
- **需同步检查：** `internal/evidence/evidence.go`、`internal/tool/builtin/todo.go`、`internal/evidence/evidence_test.go`、`internal/evidence/commandmatch_test.go`、`internal/evidence/readiness_test.go`

#### 待修改项

- [ ] 逐段核对 L418、L430、L442、L447、L453 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Ledger.UnverifiedCompletedTodos` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/evidence/evidence.go|*Ledger.UnverifiedCompletedTodos` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/evidence/evidence.go:416-465` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Ledger.UnverifiedCompletedTodos` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/evidence/evidence_test.go`。
- [ ] 运行相关包测试：`go test ./internal/evidence`。
- [ ] 运行相关包 vet：`go vet ./internal/evidence`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/evidence/evidence.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/evidence/evidence.go|*Ledger.UnverifiedCompletedTodos` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-254 · 圈复杂度 · `completeStepHasProof`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-254 |
| Baseline key | `complexity\|internal/evidence/evidence.go\|completeStepHasProof` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/evidence/evidence.go` |
| 代码行 | 718 |
| 函数/方法/包 | `completeStepHasProof` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`completeStepHasProof` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 718-756 行；可见控制流热点：L719 `if strings.TrimSpace(stringField(fields, "result")) == "" {`；L732 `if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {`；L737 `if kind == "" || strings.TrimSpace(item.Summary) == "" {`；L741 `case "verification":`；L745 `case "diff", "files":`；L749 `case "manual":`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizePaths）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function completeStepHasProof（internal/evidence/evidence.go:718-756）
- **可信生产调用方（1）：** `ReceiptFromToolCall`（`internal/evidence/evidence.go:610`，调用点 L624，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `normalizePaths`（`internal/evidence/evidence.go:928`，调用点 L746，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `stringField`（`internal/evidence/evidence.go:680`，调用点 L719，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/evidence`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`
- **需同步检查：** `internal/evidence/evidence.go`、`internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`

#### 待修改项

- [ ] 逐段核对 L719、L735、L740、L745、L749 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `completeStepHasProof` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/evidence/evidence.go|completeStepHasProof` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/evidence/evidence.go:718-756` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `completeStepHasProof` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/evidence`。
- [ ] 运行相关包 vet：`go vet ./internal/evidence`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/evidence/evidence.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/evidence/evidence.go|completeStepHasProof` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-346 · 圈复杂度 · `backfillToolCallNames`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-346 |
| Baseline key | `complexity\|internal/provider/provider.go\|backfillToolCallNames` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/provider/provider.go` |
| 代码行 | 387 |
| 函数/方法/包 | `backfillToolCallNames` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`backfillToolCallNames` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 387-423 行；可见控制流热点：L389 `for _, c := range calls {`；L395 `if !missing {`；L402 `for _, r := range results {`；L407 `for k := range out {`；L409 `if n, ok := byID[out[k].ID]; ok {`；L418 `if out[k].Name == "" && k < len(results) {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function backfillToolCallNames（internal/provider/provider.go:387-423）
- **可信生产调用方（1）：** `normalizeMessages`（`internal/provider/provider.go:148`，调用点 L165，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `idDistinct`（`internal/provider/provider.go:427`，调用点 L400，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/provider/provider_test.go`
- **同目录测试候选：** `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/image_test.go`、`internal/provider/openai/effort_test.go`、`internal/provider/openai/fetch_models_test.go`、`internal/provider/openai/host_test.go`、`internal/provider/openai/image_test.go`、`internal/provider/openai/openai_test.go`、`internal/provider/openai/realcache_test.go`、`internal/provider/openai/reconnect_test.go`、`internal/provider/openai/stall_test.go`
- **需同步检查：** `internal/provider/provider.go`、`internal/provider/provider_test.go`、`internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`

#### 待修改项

- [ ] 先冻结 `backfillToolCallNames` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L389、L400、L403、L409、L418 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `backfillToolCallNames` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/provider/provider.go|backfillToolCallNames` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/provider/provider.go:387-423` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `backfillToolCallNames` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/provider/provider_test.go`。
- [ ] 运行相关包测试：`go test ./internal/provider`。
- [ ] 运行相关包 vet：`go vet ./internal/provider`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/provider/provider.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/provider/provider.go|backfillToolCallNames` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-250 · 圈复杂度 · `*Ledger.TouchedPaths`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-250 |
| Baseline key | `complexity\|internal/evidence/evidence.go\|*Ledger.TouchedPaths` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 32） |
| 文件 | `internal/evidence/evidence.go` |
| 代码行 | 146 |
| 函数/方法/包 | `*Ledger.TouchedPaths` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Ledger.TouchedPaths` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 146-167 行；可见控制流热点：L147 `if l == nil || limit <= 0 {`；L154 `for i := len(l.receipts) - 1; i >= 0 && len(out) < limit; i-- {`；L156 `if !r.Success || (writtenOnly && !r.Write) || (!writtenOnly && !r.Read && !r.Write) {`；L159 `for _, p := range r.Paths {`；L160 `if !seen[p] && len(out) < limit {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Ledger::TouchedPaths（internal/evidence/evidence.go:146-167）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `verifyStepEvidence`（`internal/tool/builtin/completestep.go:144`，调用点 L169/L177，confidence=0.80，instance-method，2 条边，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `completeStep::Execute`（`internal/tool/builtin/completestep.go:86`，调用点 L116，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/evidence`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`
- **需同步检查：** `internal/evidence/evidence.go`、`internal/tool/builtin/completestep.go`、`internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`

#### 待修改项

- [ ] 逐段核对 L147、L154、L156、L159、L160 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `*Ledger.TouchedPaths` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/evidence/evidence.go|*Ledger.TouchedPaths` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/evidence/evidence.go:146-167` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Ledger.TouchedPaths` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/evidence/commandmatch_test.go`、`internal/evidence/evidence_test.go`、`internal/evidence/readiness_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/evidence`。
- [ ] 运行相关包 vet：`go vet ./internal/evidence`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/evidence/evidence.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/evidence/evidence.go|*Ledger.TouchedPaths` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-347 · 圈复杂度 · `currencySymbol`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-347 |
| Baseline key | `complexity\|internal/provider/provider.go\|currencySymbol` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 32） |
| 文件 | `internal/provider/provider.go` |
| 代码行 | 505 |
| 函数/方法/包 | `currencySymbol` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`currencySymbol` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 505-538 行；可见控制流热点：L507 `if value == "" {`；L511 `case "cny", "rmb", "yuan", "renminbi", "cnh":`；L517 `case "gbp", "pound", "pounds", "sterling":`；L522 `switch value {`；L529 `for _, r := range value {`；L534 `if isThreeLetterCurrencyCode(value) {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function currencySymbol（internal/provider/provider.go:505-538）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Pricing::Symbol`（`internal/provider/provider.go:498`，调用点 L502，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（1）：** `isThreeLetterCurrencyCode`（`internal/provider/provider.go:540`，调用点 L534，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/image_test.go`、`internal/provider/openai/effort_test.go`、`internal/provider/openai/fetch_models_test.go`、`internal/provider/openai/host_test.go`、`internal/provider/openai/image_test.go`、`internal/provider/openai/openai_test.go`、`internal/provider/openai/realcache_test.go`、`internal/provider/openai/reconnect_test.go`、`internal/provider/openai/stall_test.go`
- **需同步检查：** `internal/provider/provider.go`、`internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/image_test.go`

#### 待修改项

- [ ] 先冻结 `currencySymbol` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L507、L513、L519、L525、L534 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `currencySymbol` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/provider/provider.go|currencySymbol` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/provider/provider.go:505-538` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `currencySymbol` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/image_test.go`、`internal/provider/openai/effort_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/provider`。
- [ ] 运行相关包 vet：`go vet ./internal/provider`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/provider/provider.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/provider/provider.go|currencySymbol` 下降或消失且无新增/恶化。
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
