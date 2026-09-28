# G48 · internal/netclient · service 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G48 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/netclient`、`internal/equilibrium`、`internal/lsp`、`internal/i18n`、`internal/retrieval`、`internal/acp` |
| 涉及文件 | `internal/netclient/netclient.go`<br>`internal/equilibrium/convergence/global_equilibrium.go`<br>`internal/lsp/results.go`<br>`internal/i18n/i18n.go`<br>`internal/retrieval/bm25.go`<br>`internal/lsp/jsonrpc.go`<br>`internal/acp/service.go` |
| 主要治理目标 | 围绕 `internal/acp/service.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | netclient / equilibrium / lsp / i18n 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 10 条问题 |
| 建议处理顺序 | 总序第 48 组；等待 G34、G41 的同文件变更合并 |
| 前置依赖 | G34（internal/acp/service.go）；G41（internal/acp/service.go） |
| 后续验证重点 | 返回值、错误语义和现有调用顺序；并发、取消或事件顺序；协议、序列化或流式事件兼容；持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **33/100**，对象 `customProxyURL` 的分项为严重度 2.5、集中度 2.3、影响面 8.7、可执行性 20.0；最高超限 1.10x，文件内 1 条问题，CodeGraph 记录 2 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 7 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G34、G41。后续同文件组：G49，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-319` | `complexity\|internal/netclient/netclient.go\|customProxyURL` | 圈复杂度 | `internal/netclient/netclient.go` | `customProxyURL` | 228 | 11/10 | +1（1.10x） | P3 | 2 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-247` | `complexity\|internal/equilibrium/convergence/global_equilibrium.go\|applyPolicyWeights` | 圈复杂度 | `internal/equilibrium/convergence/global_equilibrium.go` | `applyPolicyWeights` | 79 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-293` | `complexity\|internal/lsp/results.go\|parseLocations` | 圈复杂度 | `internal/lsp/results.go` | `parseLocations` | 11 | 12/10 | +2（1.20x） | P3 | 2 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-271` | `complexity\|internal/i18n/i18n.go\|normalize` | 圈复杂度 | `internal/i18n/i18n.go` | `normalize` | 479 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-350` | `complexity\|internal/retrieval/bm25.go\|snippetAround` | 圈复杂度 | `internal/retrieval/bm25.go` | `snippetAround` | 169 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-292` | `complexity\|internal/lsp/jsonrpc.go\|*conn.readLoop` | 圈复杂度 | `internal/lsp/jsonrpc.go` | `*conn.readLoop` | 128 | 11/10 | +1（1.10x） | P3 | 0 可信 + 0 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-017` | `complexity\|internal/acp/service.go\|listACPMetas` | 圈复杂度 | `internal/acp/service.go` | `listACPMetas` | 1362 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 3 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-012` | `complexity\|internal/acp/service.go\|*service.sessionPrompt` | 圈复杂度 | `internal/acp/service.go` | `*service.sessionPrompt` | 525 | 11/10 | +1（1.10x） | P3 | 0 可信 + 0 待核 caller / 4 可信 + 9 待核 callee / 0 unresolved | 待修改 |
| `CX-014` | `complexity\|internal/acp/service.go\|*service.sessionList` | 圈复杂度 | `internal/acp/service.go` | `*service.sessionList` | 810 | 14/10 | +4（1.40x） | P3 | 0 可信 + 0 待核 caller / 3 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-015` | `complexity\|internal/acp/service.go\|*service.sessionDelete` | 圈复杂度 | `internal/acp/service.go` | `*service.sessionDelete` | 862 | 12/10 | +2（1.20x） | P3 | 0 可信 + 0 待核 caller / 4 可信 + 7 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-319 · 圈复杂度 · `customProxyURL`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-319 |
| Baseline key | `complexity\|internal/netclient/netclient.go\|customProxyURL` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/netclient/netclient.go` |
| 代码行 | 228 |
| 函数/方法/包 | `customProxyURL` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`customProxyURL` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 228-264 行；可见控制流热点：L229 `if raw := strings.TrimSpace(spec.URL); raw != "" {`；L234 `if err := validateProxyURL(u); err != nil {`；L243 `switch typ {`；L244 `case "http", "https", "socks5", "socks5h":`；L252 `if spec.Port <= 0 || spec.Port > 65535 {`；L257 `if spec.Password != "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（validateProxyURL）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function customProxyURL（internal/netclient/netclient.go:228-264）
- **可信生产调用方（2）：** `Summary`（`internal/netclient/netclient.go:129`，调用点 L136，confidence=0.90，exact-match）；`baseProxyFunc`（`internal/netclient/netclient.go:161`，调用点 L166，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `validateProxyURL`（`internal/netclient/netclient.go:266`，调用点 L234，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/netclient`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/netclient/netclient_test.go`
- **需同步检查：** `internal/netclient/netclient.go`、`internal/netclient/netclient_test.go`

#### 待修改项

- [ ] 逐段核对 L229、L234、L244、L252、L257 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `customProxyURL` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/netclient/netclient.go|customProxyURL` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/netclient/netclient.go:228-264` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `customProxyURL` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/netclient/netclient_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/netclient`。
- [ ] 运行相关包 vet：`go vet ./internal/netclient`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/netclient/netclient.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/netclient/netclient.go|customProxyURL` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-247 · 圈复杂度 · `applyPolicyWeights`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-247 |
| Baseline key | `complexity\|internal/equilibrium/convergence/global_equilibrium.go\|applyPolicyWeights` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 32） |
| 文件 | `internal/equilibrium/convergence/global_equilibrium.go` |
| 代码行 | 79 |
| 函数/方法/包 | `applyPolicyWeights` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`applyPolicyWeights` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 79-100 行；可见控制流热点：L80 `if report.Severity == "high" || st.OscillationIndex >= equilibriumpolicy.HighOscillationThreshold {`；L86 `if st.WindowSize >= 3 && decision.ConsensusScore < policy.ConsensusThreshold && (report.Severity ==…`；L92 `if st.WindowSize >= 4 && st.ConvergenceVelocity < equilibriumpolicy.LowConvergenceVelocity && st.Sy…`；L93 `if decision.Action != controlgraph.ActionSafeMode && decision.Action != controlgraph.ActionStabiliz…`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function applyPolicyWeights（internal/equilibrium/convergence/global_equilibrium.go:79-100）
- **可信生产调用方（1）：** `FilterDecision`（`internal/equilibrium/convergence/global_equilibrium.go:15`，调用点 L28，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `minPositive`（`internal/equilibrium/convergence/global_equilibrium.go:117`，调用点 L82/L88，confidence=0.70，exact-match，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/equilibrium`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/equilibrium/convergence/global_equilibrium_test.go`
- **需同步检查：** `internal/equilibrium/convergence/global_equilibrium.go`、`internal/equilibrium/convergence/global_equilibrium_test.go`

#### 待修改项

- [ ] 逐段核对 L80、L86、L92、L93 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `applyPolicyWeights` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/equilibrium/convergence/global_equilibrium.go|applyPolicyWeights` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/equilibrium/convergence/global_equilibrium.go:79-100` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `applyPolicyWeights` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/equilibrium/convergence/global_equilibrium_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/equilibrium/convergence`。
- [ ] 运行相关包 vet：`go vet ./internal/equilibrium/convergence`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/equilibrium/convergence/global_equilibrium.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/equilibrium/convergence/global_equilibrium.go|applyPolicyWeights` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-293 · 圈复杂度 · `parseLocations`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-293 |
| Baseline key | `complexity\|internal/lsp/results.go\|parseLocations` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 32） |
| 文件 | `internal/lsp/results.go` |
| 代码行 | 11 |
| 函数/方法/包 | `parseLocations` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`parseLocations` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 11-41 行；可见控制流热点：L13 `if s == "" || s == "null" {`；L16 `if s[0] == '[' {`；L18 `if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 && arr[0].URI != "" {`；L27 `for _, l := range links {`；L28 `if l.TargetURI != "" {`；L37 `if json.Unmarshal(raw, &one) == nil && one.URI != "" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function parseLocations（internal/lsp/results.go:11-41）
- **可信生产调用方（2）：** `Manager::Definition`（`internal/lsp/manager.go:186`，调用点 L195，confidence=0.90，exact-match）；`Manager::References`（`internal/lsp/manager.go:207`，调用点 L216，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Manager::Definition`（`internal/lsp/manager.go:186`，调用点 L195，confidence=0.90，exact-match，待源码复核）；`Manager::References`（`internal/lsp/manager.go:207`，调用点 L216，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/lsp`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/lsp/lsp_test.go`
- **同目录测试候选：** `internal/lsp/jsonrpc_framecap_test.go`、`internal/lsp/main_test.go`、`internal/lsp/manual_test.go`
- **需同步检查：** `internal/lsp/results.go`、`internal/lsp/manager.go`、`internal/lsp/lsp_test.go`、`internal/lsp/jsonrpc_framecap_test.go`、`internal/lsp/main_test.go`、`internal/lsp/manual_test.go`

#### 待修改项

- [ ] 逐段核对 L13、L18、L25、L28、L37 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `parseLocations` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/lsp/results.go|parseLocations` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/lsp/results.go:11-41` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `parseLocations` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/lsp/lsp_test.go`。
- [ ] 运行相关包测试：`go test ./internal/lsp`。
- [ ] 运行相关包 vet：`go vet ./internal/lsp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/lsp/results.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/lsp/results.go|parseLocations` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-271 · 圈复杂度 · `normalize`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-271 |
| Baseline key | `complexity\|internal/i18n/i18n.go\|normalize` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 31） |
| 文件 | `internal/i18n/i18n.go` |
| 代码行 | 479 |
| 函数/方法/包 | `normalize` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`normalize` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 479-495 行；可见控制流热点：L482 `if s == "" {`；L485 `if strings.HasPrefix(s, "zh-tw") || strings.HasPrefix(s, "zh-hant") || strings.Contains(s, "chinese…`；L488 `if strings.HasPrefix(s, "zh") || strings.Contains(s, "chinese") || strings.Contains(s, "中文") {`；L491 `if strings.HasPrefix(s, "en") || strings.Contains(s, "english") {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function normalize（internal/i18n/i18n.go:479-495）
- **可信生产调用方（1）：** `DetectLanguage`（`internal/i18n/i18n.go:444`，调用点 L446，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L55，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/i18n`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/i18n/i18n_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/i18n/i18n.go`、`internal/i18n/i18n_test.go`

#### 待修改项

- [ ] 逐段核对 L482、L485、L488、L491 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `normalize` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/i18n/i18n.go|normalize` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/i18n/i18n.go:479-495` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `normalize` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/i18n/i18n_test.go`。
- [ ] 运行相关包测试：`go test ./internal/i18n`。
- [ ] 运行相关包 vet：`go vet ./internal/i18n`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/i18n/i18n.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/i18n/i18n.go|normalize` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-350 · 圈复杂度 · `snippetAround`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-350 |
| Baseline key | `complexity\|internal/retrieval/bm25.go\|snippetAround` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 31） |
| 文件 | `internal/retrieval/bm25.go` |
| 代码行 | 169 |
| 函数/方法/包 | `snippetAround` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`snippetAround` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 169-202 行；可见控制流热点：L170 `if byteIdx < 0 {`；L173 `if byteIdx > len(text) {`；L182 `if start < 0 {`；L186 `if end > len(runes) {`；L195 `if start > 0 {`；L198 `if end < len(runes) {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function snippetAround（internal/retrieval/bm25.go:169-202）
- **可信生产调用方（1）：** `MakeSnippet`（`internal/retrieval/bm25.go:140`，调用点 L166，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/retrieval`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/retrieval/bm25_test.go`
- **需同步检查：** `internal/retrieval/bm25.go`、`internal/retrieval/bm25_test.go`

#### 待修改项

- [ ] 逐段核对 L170、L176、L186、L189、L198 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `snippetAround` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/retrieval/bm25.go|snippetAround` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/retrieval/bm25.go:169-202` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `snippetAround` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/retrieval/bm25_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/retrieval`。
- [ ] 运行相关包 vet：`go vet ./internal/retrieval`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/retrieval/bm25.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/retrieval/bm25.go|snippetAround` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-292 · 圈复杂度 · `*conn.readLoop`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-292 |
| Baseline key | `complexity\|internal/lsp/jsonrpc.go\|*conn.readLoop` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 30） |
| 文件 | `internal/lsp/jsonrpc.go` |
| 代码行 | 128 |
| 函数/方法/包 | `*conn.readLoop` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*conn.readLoop` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 128-158 行；可见控制流热点：L130 `for {`；L137 `if json.Unmarshal(body, &m) != nil {`；L141 `case m.Method != "" && m.ID != nil:`；L142 `if c.onRequest != nil {`；L146 `if c.onNotify != nil {`；L153 `if ch != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readFrame）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method conn::readLoop（internal/lsp/jsonrpc.go:128-158）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `readFrame`（`internal/lsp/jsonrpc.go:175`，调用点 L131，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `conn::fail`（`internal/lsp/jsonrpc.go:160`，调用点 L133，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/lsp`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/lsp/jsonrpc_framecap_test.go`、`internal/lsp/lsp_test.go`、`internal/lsp/main_test.go`、`internal/lsp/manual_test.go`
- **需同步检查：** `internal/lsp/jsonrpc.go`、`internal/lsp/jsonrpc_framecap_test.go`、`internal/lsp/lsp_test.go`、`internal/lsp/main_test.go`、`internal/lsp/manual_test.go`

#### 待修改项

- [ ] 逐段核对 L130、L137、L142、L146、L153 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*conn.readLoop` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/lsp/jsonrpc.go|*conn.readLoop` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/lsp/jsonrpc.go:128-158` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*conn.readLoop` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/lsp/jsonrpc_framecap_test.go`、`internal/lsp/lsp_test.go`、`internal/lsp/main_test.go`、`internal/lsp/manual_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/lsp`。
- [ ] 运行相关包 race：`go test -race ./internal/lsp`。
- [ ] 运行相关包 vet：`go vet ./internal/lsp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/lsp/jsonrpc.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/lsp/jsonrpc.go|*conn.readLoop` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-017 · 圈复杂度 · `listACPMetas`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-017 |
| Baseline key | `complexity\|internal/acp/service.go\|listACPMetas` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 28） |
| 文件 | `internal/acp/service.go` |
| 代码行 | 1362 |
| 函数/方法/包 | `listACPMetas` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`listACPMetas` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 1362-1396 行；可见控制流热点：L1364 `if err != nil {`；L1371 `for _, e := range entries {`；L1372 `if e.IsDir() || !strings.HasSuffix(e.Name(), ".acp.json") {`；L1380 `if !sessionFileExists(sessionPath) {`；L1384 `if err != nil || !ok {`；L1390 `if meta.Cwd == "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadACPMeta）、持久化/I-O（sessionFileExists）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function listACPMetas（internal/acp/service.go:1362-1396）
- **可信生产调用方（1）：** `service::sessionList`（`internal/acp/service.go:810`，调用点 L827，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `loadACPMeta`（`internal/acp/service.go:1303`，调用点 L1383，confidence=0.90，exact-match）；`sessionFileExists`（`internal/acp/service.go:1398`，调用点 L1380，confidence=0.90，exact-match）；`IsCleanupPending`（`internal/agent/save.go:256`，调用点 L1377，confidence=0.90，import）
- **待源码复核被调候选（1）：** `transcriptPath`（`internal/acp/service.go:517`，调用点 L1376，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `service::sessionList`（`internal/acp/service.go:810`，调用点 L827，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/acp`、`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/acp/server_test.go`
- **同目录测试候选：** `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`
- **需同步检查：** `internal/acp/service.go`、`internal/agent/save.go`、`internal/acp/server_test.go`、`internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`

#### 待修改项

- [ ] 先冻结 `listACPMetas` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1364、L1371、L1377、L1384、L1390 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `listACPMetas` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/acp/service.go|listACPMetas` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/acp/service.go:1362-1396` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `listACPMetas` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/acp/server_test.go`。
- [ ] 运行相关包测试：`go test ./internal/acp`。
- [ ] 运行相关包 vet：`go vet ./internal/acp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/acp/service.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/acp/service.go|listACPMetas` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-012 · 圈复杂度 · `*service.sessionPrompt`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-012 |
| Baseline key | `complexity\|internal/acp/service.go\|*service.sessionPrompt` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 27） |
| 文件 | `internal/acp/service.go` |
| 代码行 | 525 |
| 函数/方法/包 | `*service.sessionPrompt` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*service.sessionPrompt` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 525-577 行；可见控制流热点：L527 `if err := json.Unmarshal(raw, &p); err != nil {`；L535 `if text == "" {`；L548 `if err := s.applyPendingSessionConfig(ctx, sess); err != nil {`；L550 `if isSessionConfigActiveWorkError(err) {`；L565 `if runErr != nil {`；L573 `if sess.transcript != "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（isSessionConfigActiveWorkError、service::applyPendingSessionConfig、service::configStateForSession、service::resolveSlashPrompt）、状态与控制（updateSink::setTurnContext、updateSink::clearTurnContext、updateSink::Emit、updateSink::send）、持久化/I-O（acpSession::persistAfterTurn、ArchiveSink::Err）、渲染/输出（updateSink::Emit）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method service::sessionPrompt（internal/acp/service.go:525-577）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `updateSink::setTurnContext`（`internal/acp/dispatch.go:75`，调用点 L544，confidence=0.90，exact-match）；`updateSink::clearTurnContext`（`internal/acp/dispatch.go:81`，调用点 L546，confidence=0.90，exact-match）；`FlattenPrompt`（`internal/acp/protocol.go:337`，调用点 L534，confidence=0.90，exact-match）；`isSessionConfigActiveWorkError`（`internal/acp/service.go:786`，调用点 L550，confidence=0.90，exact-match）
- **待源码复核被调候选（9）：** `updateSink::Emit`（`internal/acp/dispatch.go:99`，调用点 L549，confidence=0.70，exact-match，待源码复核）；`updateSink::send`（`internal/acp/dispatch.go:178`，调用点 L552，confidence=0.70，exact-match，待源码复核）；`acpSession::finish`（`internal/acp/service.go:194`，调用点 L547，confidence=0.70，instance-method，待源码复核）；`service::applyPendingSessionConfig`（`internal/acp/service.go:745`，调用点 L548，confidence=0.70，instance-method，待源码复核）；`service::session`（`internal/acp/service.go:917`，调用点 L530，confidence=0.70，instance-method，待源码复核）；`service::configStateForSession`（`internal/acp/service.go:964`，调用点 L551，confidence=0.70，instance-method，待源码复核）；`acpSession::persistAfterTurn`（`internal/acp/service.go:1102`，调用点 L562，confidence=0.70，instance-method，待源码复核）；`service::resolveSlashPrompt`（`internal/acp/service.go:1228`，调用点 L538，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/acp`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`、`internal/acp/server_test.go`
- **需同步检查：** `internal/acp/service.go`、`internal/acp/dispatch.go`、`internal/acp/protocol.go`、`internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`

#### 待修改项

- [ ] 先冻结 `*service.sessionPrompt` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L527、L535、L550、L565、L573 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*service.sessionPrompt` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/acp/service.go|*service.sessionPrompt` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/acp/service.go:525-577` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*service.sessionPrompt` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/acp`。
- [ ] 运行相关包 race：`go test -race ./internal/acp`。
- [ ] 运行相关包 vet：`go vet ./internal/acp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/acp/service.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/acp/service.go|*service.sessionPrompt` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-014 · 圈复杂度 · `*service.sessionList`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-014 |
| Baseline key | `complexity\|internal/acp/service.go\|*service.sessionList` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 27） |
| 文件 | `internal/acp/service.go` |
| 代码行 | 810 |
| 函数/方法/包 | `*service.sessionList` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*service.sessionList` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 810-858 行；可见控制流热点：L812 `if len(raw) > 0 {`；L818 `if filterCwd != "" && !filepath.IsAbs(filterCwd) {`；L826 `if dir := s.sessionDir(); dir != "" {`；L833 `if sessionInfoMatchesCwd(info, filterCwd) {`；L840 `if sessionInfoMatchesCwd(info, filterCwd) {`；L852 `if ti.Equal(tj) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseSessionUpdatedAt）、状态与控制（parseSessionUpdatedAt）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method service::sessionList（internal/acp/service.go:810-858）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `sessionInfoMatchesCwd`（`internal/acp/service.go:1418`，调用点 L833/L840，confidence=0.90，exact-match，2 条边）；`parseSessionUpdatedAt`（`internal/acp/service.go:1475`，调用点 L850/L851，confidence=0.90，exact-match，2 条边）；`listACPMetas`（`internal/acp/service.go:1362`，调用点 L827，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `service::liveSessions`（`internal/acp/service.go:931`，调用点 L838，confidence=0.70，instance-method，待源码复核）；`service::sessionDir`（`internal/acp/service.go:941`，调用点 L826，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/acp`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`、`internal/acp/server_test.go`
- **需同步检查：** `internal/acp/service.go`、`internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`

#### 待修改项

- [ ] 先冻结 `*service.sessionList` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L812、L821、L831、L838、L852 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*service.sessionList` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/acp/service.go|*service.sessionList` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/acp/service.go:810-858` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*service.sessionList` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/acp`。
- [ ] 运行相关包 vet：`go vet ./internal/acp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/acp/service.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/acp/service.go|*service.sessionList` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-015 · 圈复杂度 · `*service.sessionDelete`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-015 |
| Baseline key | `complexity\|internal/acp/service.go\|*service.sessionDelete` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 27） |
| 文件 | `internal/acp/service.go` |
| 代码行 | 862 |
| 函数/方法/包 | `*service.sessionDelete` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*service.sessionDelete` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 862-903 行；可见控制流热点：L864 `if err := json.Unmarshal(raw, &p); err != nil {`；L874 `if sess := s.takeSession(p.SessionID); sess != nil {`；L879 `if err := agent.MarkCleanupPending(path, "delete"); err != nil {`；L889 `if path == "" {`；L894 `if path != "" && !delayed {`；L898 `if destroy.Finish != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（validateSessionID）、持久化/I-O（delayedDeleteSessionFiles、deleteSessionFiles）、任务/并发（acpSession::deleteAndWait、Manager::Wait）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method service::sessionDelete（internal/acp/service.go:862-903）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `delayedDeleteSessionFiles`（`internal/acp/service.go:1520`，调用点 L880/L884，confidence=0.90，exact-match，2 条边）；`validateSessionID`（`internal/acp/service.go:1445`，调用点 L867，confidence=0.90，exact-match）；`deleteSessionFiles`（`internal/acp/service.go:1483`，调用点 L895，confidence=0.90，exact-match）；`MarkCleanupPending`（`internal/agent/save.go:227`，调用点 L879，confidence=0.90，import）
- **待源码复核被调候选（7）：** `acpSession::deleteAndWait`（`internal/acp/service.go:228`，调用点 L875，confidence=0.70，instance-method，待源码复核）；`transcriptPath`（`internal/acp/service.go:517`，调用点 L891，confidence=0.70，exact-match，待源码复核）；`service::takeSession`（`internal/acp/service.go:923`，调用点 L874，confidence=0.70，instance-method，待源码复核）；`service::sessionDir`（`internal/acp/service.go:941`，调用点 L890，confidence=0.70，instance-method，待源码复核）；`TeardownResult::HasTimedOut`（`internal/jobs/jobs.go:77`，调用点 L878，confidence=0.80，instance-method，待源码复核）；`Manager::Wait`（`internal/jobs/jobs.go:702`，调用点 L878，confidence=0.70，instance-method，待源码复核）；`Turn::Finish`（`internal/memorycompiler/runtime.go:1728`，调用点 L899，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/acp`、`internal/agent`、`internal/jobs`、`internal/memorycompiler`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`、`internal/acp/server_test.go`
- **需同步检查：** `internal/acp/service.go`、`internal/agent/save.go`、`internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`

#### 待修改项

- [ ] 先冻结 `*service.sessionDelete` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L864、L874、L889、L894、L898 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*service.sessionDelete` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/acp/service.go|*service.sessionDelete` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/acp/service.go:862-903` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*service.sessionDelete` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G49 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/acp`。
- [ ] 运行相关包 race：`go test -race ./internal/acp`。
- [ ] 运行相关包 vet：`go vet ./internal/acp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/acp/service.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/acp/service.go|*service.sessionDelete` 下降或消失且无新增/恶化。
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
