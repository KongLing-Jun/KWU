# G41 · cmd/e2ebench · diff 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G41 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `cmd/e2ebench`、`internal/acp` |
| 涉及文件 | `cmd/e2ebench/diff.go`<br>`internal/acp/dispatch.go`<br>`cmd/e2ebench/main.go`<br>`cmd/e2ebench/mutation.go`<br>`internal/acp/service.go` |
| 主要治理目标 | 围绕 `internal/acp/service.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | cmd/e2ebench / acp 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 10 条问题 |
| 建议处理顺序 | 总序第 41 组；等待 G27、G34 的同文件变更合并 |
| 前置依赖 | G27（cmd/e2ebench/diff.go）；G34（internal/acp/service.go） |
| 后续验证重点 | 返回值、错误语义和现有调用顺序；并发、取消或事件顺序；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；持久化、迁移或文件格式兼容；配置、平台或向后兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **39/100**，对象 `changedLineCoverage` 的分项为严重度 6.3、集中度 6.4、影响面 10.4、可执行性 16.0；最高超限 1.25x，文件内 6 条问题，CodeGraph 记录 1 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 5 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G27、G34。后续同文件组：G48、G49，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `ND-001` | `nesting_depth\|cmd/e2ebench/diff.go\|changedLineCoverage` | 嵌套深度 | `cmd/e2ebench/diff.go` | `changedLineCoverage` | 345 | 5/4 | +1（1.25x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-010` | `complexity\|internal/acp/dispatch.go\|toolKindFor` | 圈复杂度 | `internal/acp/dispatch.go` | `toolKindFor` | 380 | 18/10 | +8（1.80x） | P3 | 3 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-009` | `complexity\|internal/acp/dispatch.go\|*updateSink.Emit` | 圈复杂度 | `internal/acp/dispatch.go` | `*updateSink.Emit` | 99 | 16/10 | +6（1.60x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 5 待核 callee / 0 unresolved | 待修改 |
| `CX-005` | `complexity\|cmd/e2ebench/main.go\|main` | 圈复杂度 | `cmd/e2ebench/main.go` | `main` | 48 | 13/10 | +3（1.30x） | P3 | 0 可信 + 0 待核 caller / 2 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `CX-006` | `complexity\|cmd/e2ebench/main.go\|render` | 圈复杂度 | `cmd/e2ebench/main.go` | `render` | 240 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-007` | `complexity\|cmd/e2ebench/mutation.go\|runMutation` | 圈复杂度 | `cmd/e2ebench/mutation.go` | `runMutation` | 28 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-003` | `complexity\|cmd/e2ebench/diff.go\|parseNewTests` | 圈复杂度 | `cmd/e2ebench/diff.go` | `parseNewTests` | 477 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-004` | `complexity\|cmd/e2ebench/diff.go\|splitShellFields` | 圈复杂度 | `cmd/e2ebench/diff.go` | `splitShellFields` | 562 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-013` | `complexity\|internal/acp/service.go\|*service.rebuildSession` | 圈复杂度 | `internal/acp/service.go` | `*service.rebuildSession` | 667 | 14/10 | +4（1.40x） | P3 | 0 可信 + 3 待核 caller / 5 可信 + 8 待核 callee / 0 unresolved | 待修改 |
| `CX-016` | `complexity\|internal/acp/service.go\|availableCommandsFor` | 圈复杂度 | `internal/acp/service.go` | `availableCommandsFor` | 1165 | 16/10 | +6（1.60x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### ND-001 · 嵌套深度 · `changedLineCoverage`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-001 |
| Baseline key | `nesting_depth\|cmd/e2ebench/diff.go\|changedLineCoverage` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `cmd/e2ebench/diff.go` |
| 代码行 | 345 |
| 函数/方法/包 | `changedLineCoverage` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`changedLineCoverage` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L354 `for file, lines := range changedLineSet(repo, base, srcFiles) {`；L356 `for ln := range lines {`；L357 `for _, blk := range fileBlocks {`；L358 `if ln >= blk.start && ln <= blk.end {`；L360 `if blk.count > 0 {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function changedLineCoverage（cmd/e2ebench/diff.go:345-369）
- **可信生产调用方（1）：** `runOnce`（`cmd/e2ebench/diff.go:69`，调用点 L103，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `parseCoverProfile`（`cmd/e2ebench/diff.go:377`，调用点 L353，confidence=0.90，exact-match）；`changedLineSet`（`cmd/e2ebench/diff.go:418`，调用点 L354，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`cmd/e2ebench`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `cmd/e2ebench/diff.go`

#### 待修改项

- [ ] 从 L354、L356、L357、L358、L360 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|cmd/e2ebench/diff.go|changedLineCoverage` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`cmd/e2ebench/diff.go:345-369` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `changedLineCoverage` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G48、G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 补充或确认覆盖当前职责、错误分支和边界条件的聚焦测试；不得仅依赖全量测试。
- [ ] 运行相关包测试：`go test ./cmd/e2ebench`。
- [ ] 运行相关包 vet：`go vet ./cmd/e2ebench`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- cmd/e2ebench/diff.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|cmd/e2ebench/diff.go|changedLineCoverage` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-010 · 圈复杂度 · `toolKindFor`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-010 |
| Baseline key | `complexity\|internal/acp/dispatch.go\|toolKindFor` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/acp/dispatch.go` |
| 代码行 | 380 |
| 函数/方法/包 | `toolKindFor` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`toolKindFor` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 380-404 行；可见控制流热点：L381 `switch name {`；L384 `case "grep":`；L388 `case "bash":`；L392 `switch {`；L395 `case strings.Contains(n, "edit") || strings.Contains(n, "write") || strings.Contains(n, "replace"):`；L399 `case strings.Contains(n, "bash") || strings.Contains(n, "exec") || strings.Contains(n, "shell") || …`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function toolKindFor（internal/acp/dispatch.go:380-404）
- **可信生产调用方（3）：** `updateSink::Emit`（`internal/acp/dispatch.go:99`，调用点 L123，confidence=0.90，exact-match）；`updateSink::replay`（`internal/acp/dispatch.go:186`，调用点 L205，confidence=0.90，exact-match）；`updateSink::requestPermission`（`internal/acp/dispatch.go:225`，调用点 L239，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `service::sessionPrompt`（`internal/acp/service.go:525`，调用点 L549，confidence=0.70，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/acp`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/acp/protocol_test.go`
- **同目录测试候选：** `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/server_test.go`
- **需同步检查：** `internal/acp/dispatch.go`、`internal/acp/protocol_test.go`、`internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`

#### 待修改项

- [ ] 逐段核对 L381、L384、L392、L395、L399 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `toolKindFor` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/acp/dispatch.go|toolKindFor` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/acp/dispatch.go:380-404` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `toolKindFor` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G48、G49 处理。
- 重点保护：协议、序列化或流式事件兼容；权限、安全或凭据边界。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/acp/protocol_test.go`。
- [ ] 运行相关包测试：`go test ./internal/acp`。
- [ ] 运行相关包 vet：`go vet ./internal/acp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/acp/dispatch.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/acp/dispatch.go|toolKindFor` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-009 · 圈复杂度 · `*updateSink.Emit`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-009 |
| Baseline key | `complexity\|internal/acp/dispatch.go\|*updateSink.Emit` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 38） |
| 文件 | `internal/acp/dispatch.go` |
| 代码行 | 99 |
| 函数/方法/包 | `*updateSink.Emit` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*updateSink.Emit` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 99-176 行；可见控制流热点：L100 `switch e.Kind {`；L107 `case event.Text:`；L116 `if e.Tool.Partial {`；L131 `if e.Tool.Err != "" {`；L152 `case event.CompactionDone:`；L169 `case event.AskRequest:`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（updateSink::currentTurnContext、updateSink::requestPermission、updateSink::requestAsk）、网络/协议（updateSink::requestPermission、updateSink::requestAsk）、权限/安全（updateSink::requestPermission）、任务/并发（textBlock、updateSink::requestAsk）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method updateSink::Emit（internal/acp/dispatch.go:99-176）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `service::sessionPrompt`（`internal/acp/service.go:525`，调用点 L549，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（2）：** `rawJSON`（`internal/acp/dispatch.go:356`，调用点 L125，confidence=0.90，exact-match）；`toolKindFor`（`internal/acp/dispatch.go:380`，调用点 L123，confidence=0.90，exact-match）
- **待源码复核被调候选（5）：** `textBlock`（`internal/acp/dispatch.go:352`，调用点 L105/L111/L139/L148，confidence=0.70，exact-match，5 条边，待源码复核）；`updateSink::currentTurnContext`（`internal/acp/dispatch.go:87`，调用点 L166/L173，confidence=0.70，instance-method，2 条边，待源码复核）；`updateSink::requestPermission`（`internal/acp/dispatch.go:225`，调用点 L167，confidence=0.70，instance-method，待源码复核）；`updateSink::requestAsk`（`internal/acp/dispatch.go:262`，调用点 L174，confidence=0.70，instance-method，待源码复核）；`clip`（`internal/acp/dispatch.go:364`，调用点 L139，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `service::sessionPrompt`（`internal/acp/service.go:525`，调用点 L549，confidence=0.70，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/acp`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`、`internal/acp/server_test.go`
- **需同步检查：** `internal/acp/dispatch.go`、`internal/acp/service.go`、`internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`

#### 待修改项

- [ ] 逐段核对 L100、L108、L128、L152、L169 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、网络/协议、权限/安全 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*updateSink.Emit` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/acp/dispatch.go|*updateSink.Emit` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/acp/dispatch.go:99-176` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*updateSink.Emit` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G48、G49 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/acp`。
- [ ] 运行相关包 race：`go test -race ./internal/acp`。
- [ ] 运行相关包 vet：`go vet ./internal/acp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/acp/dispatch.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/acp/dispatch.go|*updateSink.Emit` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-005 · 圈复杂度 · `main`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-005 |
| Baseline key | `complexity\|cmd/e2ebench/main.go\|main` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `cmd/e2ebench/main.go` |
| 代码行 | 48 |
| 函数/方法/包 | `main` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`main` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 48-132 行；可见控制流热点：L76 `if *mode == "diff" {`；L90 `if len(tasks) == 0 {`；L102 `for _, t := range tasks {`；L113 `if *outMD != "" {`；L121 `if *outJSON != "" {`；L127 `if err := os.WriteFile(*outJSON, b, 0o644); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadTasks）、状态与控制（runDiff、runTask）、渲染/输出（emit、render）、任务/并发（loadTasks、runTask）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function main（cmd/e2ebench/main.go:48-132）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `Manager::Output`（`internal/jobs/jobs.go:575`，调用点 L50/L51/L53/L54，confidence=0.90，exact-match，7 条边）；`runDiff`（`cmd/e2ebench/diff.go:35`，调用点 L77，confidence=0.90，exact-match）
- **待源码复核被调候选（4）：** `emit`（`cmd/e2ebench/main.go:134`，调用点 L81，confidence=0.70，exact-match，待源码复核）；`loadTasks`（`cmd/e2ebench/main.go:145`，调用点 L85，confidence=0.70，exact-match，待源码复核）；`runTask`（`cmd/e2ebench/main.go:175`，调用点 L107，confidence=0.70，exact-match，待源码复核）；`render`（`cmd/e2ebench/main.go:240`，调用点 L112，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`cmd/e2ebench`、`internal/jobs`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `cmd/e2ebench/main.go`、`internal/jobs/jobs.go`、`cmd/e2ebench/diff.go`

#### 待修改项

- [ ] 逐段核对 L76、L92、L103、L121、L127 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `main` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|cmd/e2ebench/main.go|main` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`cmd/e2ebench/main.go:48-132` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `main` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G48、G49 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 补充或确认覆盖当前职责、错误分支和边界条件的聚焦测试；不得仅依赖全量测试。
- [ ] 运行相关包测试：`go test ./cmd/e2ebench`。
- [ ] 运行相关包 race：`go test -race ./cmd/e2ebench`。
- [ ] 运行相关包 vet：`go vet ./cmd/e2ebench`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- cmd/e2ebench/main.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|cmd/e2ebench/main.go|main` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-006 · 圈复杂度 · `render`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-006 |
| Baseline key | `complexity\|cmd/e2ebench/main.go\|render` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `cmd/e2ebench/main.go` |
| 代码行 | 240 |
| 函数/方法/包 | `render` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`render` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 240-303 行；可见控制流热点：L246 `for _, r := range results {`；L251 `if r.Passed {`；L272 `for _, r := range results {`；L278 `if r.Passed {`；L291 `if r.Note != "" {`；L299 `if notes {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function render（cmd/e2ebench/main.go:240-303）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `main`（`cmd/e2ebench/main.go:48`，调用点 L112，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（2）：** `comma`（`cmd/e2ebench/main.go:312`，调用点 L268/L282，confidence=0.90，exact-match，5 条边）；`currencySym`（`cmd/e2ebench/main.go:327`，调用点 L268/L284，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（1）：** `pct`（`cmd/e2ebench/main.go:305`，调用点 L267/L283，confidence=0.70，exact-match，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** `main`（`cmd/e2ebench/main.go:48`，调用点 L112，confidence=0.70，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`cmd/e2ebench`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `cmd/e2ebench/main.go`

#### 待修改项

- [ ] 逐段核对 L246、L260、L274、L290、L299 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `render` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|cmd/e2ebench/main.go|render` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`cmd/e2ebench/main.go:240-303` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `render` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G48、G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 补充或确认覆盖当前职责、错误分支和边界条件的聚焦测试；不得仅依赖全量测试。
- [ ] 运行相关包测试：`go test ./cmd/e2ebench`。
- [ ] 运行相关包 vet：`go vet ./cmd/e2ebench`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- cmd/e2ebench/main.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|cmd/e2ebench/main.go|render` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-007 · 圈复杂度 · `runMutation`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-007 |
| Baseline key | `complexity\|cmd/e2ebench/mutation.go\|runMutation` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `cmd/e2ebench/mutation.go` |
| 代码行 | 28 |
| 函数/方法/包 | `runMutation` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`runMutation` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 28-91 行；可见控制流热点：L31 `for _, r := range refs {`；L37 `if res.total >= maxMutants {`；L49 `if err != nil {`；L58 `for _, fd := range changedFuncs(fset, f, changed[file]) {`；L66 `if os.WriteFile(abs, []byte(mutated), 0o644) != nil {`；L83 `if caught {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（changedLineSet）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function runMutation（cmd/e2ebench/mutation.go:28-91）
- **可信生产调用方（1）：** `runOnce`（`cmd/e2ebench/diff.go:69`，调用点 L105，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `changedLineSet`（`cmd/e2ebench/diff.go:418`，调用点 L29，confidence=0.90，exact-match）；`changedFuncs`（`cmd/e2ebench/mutation.go:95`，调用点 L58，confidence=0.90，exact-match）；`mutantBody`（`cmd/e2ebench/mutation.go:121`，调用点 L65，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`cmd/e2ebench`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `cmd/e2ebench/mutation.go`、`cmd/e2ebench/diff.go`

#### 待修改项

- [ ] 逐段核对 L31、L42、L54、L66、L83 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `runMutation` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|cmd/e2ebench/mutation.go|runMutation` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`cmd/e2ebench/mutation.go:28-91` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `runMutation` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G48、G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 补充或确认覆盖当前职责、错误分支和边界条件的聚焦测试；不得仅依赖全量测试。
- [ ] 运行相关包测试：`go test ./cmd/e2ebench`。
- [ ] 运行相关包 vet：`go vet ./cmd/e2ebench`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- cmd/e2ebench/mutation.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|cmd/e2ebench/mutation.go|runMutation` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-003 · 圈复杂度 · `parseNewTests`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-003 |
| Baseline key | `complexity\|cmd/e2ebench/diff.go\|parseNewTests` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `cmd/e2ebench/diff.go` |
| 代码行 | 477 |
| 函数/方法/包 | `parseNewTests` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`parseNewTests` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 477-518 行；可见控制流热点：L480 `for _, ln := range strings.Split(diff, "\n") {`；L485 `if !strings.HasPrefix(ln, "+") || strings.HasPrefix(ln, "+++") {`；L489 `if !strings.HasPrefix(body, "func ") {`；L497 `if close < 0 {`；L502 `if methodParen <= 0 {`；L513 `if strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Fuzz") || strings.HasPrefix(name, "B…`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function parseNewTests（cmd/e2ebench/diff.go:477-518）
- **可信生产调用方（1）：** `runOnce`（`cmd/e2ebench/diff.go:69`，调用点 L95，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`cmd/e2ebench`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `cmd/e2ebench/diff.go`

#### 待修改项

- [ ] 逐段核对 L480、L485、L495、L502、L513 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `parseNewTests` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|cmd/e2ebench/diff.go|parseNewTests` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`cmd/e2ebench/diff.go:477-518` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `parseNewTests` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G48、G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 补充或确认覆盖当前职责、错误分支和边界条件的聚焦测试；不得仅依赖全量测试。
- [ ] 运行相关包测试：`go test ./cmd/e2ebench`。
- [ ] 运行相关包 vet：`go vet ./cmd/e2ebench`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- cmd/e2ebench/diff.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|cmd/e2ebench/diff.go|parseNewTests` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-004 · 圈复杂度 · `splitShellFields`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-004 |
| Baseline key | `complexity\|cmd/e2ebench/diff.go\|splitShellFields` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `cmd/e2ebench/diff.go` |
| 代码行 | 562 |
| 函数/方法/包 | `splitShellFields` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`splitShellFields` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 562-598 行；可见控制流热点：L566 `for i := 0; i < len(s); i++ {`；L569 `case inSingle:`；L575 `case inDouble:`；L581 `case c == '\'':`；L585 `case c == ' ' || c == '\t' || c == '\n':`；L594 `if cur.Len() > 0 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function splitShellFields（cmd/e2ebench/diff.go:562-598）
- **可信生产调用方（1）：** `runTests`（`cmd/e2ebench/diff.go:548`，调用点 L549，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `Registry::Len`（`internal/tool/tool.go:242`，调用点 L586/L594，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`cmd/e2ebench`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `cmd/e2ebench/diff.go`、`internal/tool/tool.go`

#### 待修改项

- [ ] 逐段核对 L566、L570、L576、L585、L594 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `splitShellFields` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|cmd/e2ebench/diff.go|splitShellFields` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`cmd/e2ebench/diff.go:562-598` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `splitShellFields` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G48、G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 补充或确认覆盖当前职责、错误分支和边界条件的聚焦测试；不得仅依赖全量测试。
- [ ] 运行相关包测试：`go test ./cmd/e2ebench`。
- [ ] 运行相关包 vet：`go vet ./cmd/e2ebench`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- cmd/e2ebench/diff.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|cmd/e2ebench/diff.go|splitShellFields` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-013 · 圈复杂度 · `*service.rebuildSession`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-013 |
| Baseline key | `complexity\|internal/acp/service.go\|*service.rebuildSession` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 30） |
| 文件 | `internal/acp/service.go` |
| 代码行 | 667 |
| 函数/方法/包 | `*service.rebuildSession` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*service.rebuildSession` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 667-743 行；可见控制流热点：L669 `if sess.deleted {`；L678 `if !sess.running && !status.Running && status.BackgroundJobs > 0 {`；L682 `if sess.running || status.Running {`；L698 `if err := cur.Snapshot(); err != nil {`；L716 `if err != nil {`；L734 `if sess.transcript != "" && sessionFileExists(sess.transcript) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（sessionConfigActiveWorkError、cloneSessionConfigState）、状态与控制（cloneSessionConfigState、updateSink::bindApprove、updateSink::bindAnswer、updateSink::send）、持久化/I-O（saveACPMeta、sessionFileExists）、权限/安全（updateSink::bindApprove）、任务/并发（acpSession::metaLocked）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method service::rebuildSession（internal/acp/service.go:667-743）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（3）：** `service::switchSessionModel`（`internal/acp/service.go:634`，调用点 L644，confidence=0.70，instance-method，待源码复核）；`service::switchSessionEffort`（`internal/acp/service.go:650`，调用点 L661，confidence=0.70，instance-method，待源码复核）；`service::applyPendingSessionConfig`（`internal/acp/service.go:745`，调用点 L758，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（5）：** `sessionConfigActiveWorkError`（`internal/acp/service.go:780`，调用点 L676/L680，confidence=0.90，exact-match，2 条边）；`cloneSessionConfigState`（`internal/acp/service.go:1030`，调用点 L683，confidence=0.90，exact-match）；`clonePluginSpecs`（`internal/acp/service.go:1045`，调用点 L704，confidence=0.90，exact-match）；`saveACPMeta`（`internal/acp/service.go:1322`，调用点 L735，confidence=0.90，exact-match）；`sessionFileExists`（`internal/acp/service.go:1398`，调用点 L734，confidence=0.90，exact-match）
- **待源码复核被调候选（8）：** `cloneStringPtr`（`internal/acp/service.go:1022`，调用点 L712/L733，confidence=0.70，exact-match，2 条边，待源码复核）；`updateSink::bindApprove`（`internal/acp/dispatch.go:61`，调用点 L721，confidence=0.70，instance-method，待源码复核）；`updateSink::bindAnswer`（`internal/acp/dispatch.go:71`，调用点 L722，confidence=0.70，instance-method，待源码复核）；`updateSink::send`（`internal/acp/dispatch.go:178`，调用点 L686，confidence=0.70，exact-match，待源码复核）；`Factory::NewSession`（`internal/acp/service.go:59`，调用点 L707，confidence=0.70，exact-match，待源码复核）；`acpSession::metaLocked`（`internal/acp/service.go:1127`，调用点 L735，confidence=0.70，instance-method，待源码复核）；`service::sendAvailableCommands`（`internal/acp/service.go:1151`，调用点 L740，confidence=0.70，instance-method，待源码复核）；`Controller::AdoptHistory`（`internal/control/sessionpath.go:31`，调用点 L723，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `service::sessionPrompt`（`internal/acp/service.go:525`，调用点 L548，confidence=0.70，instance-method，待源码复核）；`service::sessionSetConfigOption`（`internal/acp/service.go:581`，调用点 L605，confidence=0.70，instance-method，待源码复核）；`service::sessionSetModel`（`internal/acp/service.go:619`，调用点 L628，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/acp`、`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`、`internal/acp/server_test.go`
- **需同步检查：** `internal/acp/service.go`、`internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`

#### 待修改项

- [ ] 先冻结 `*service.rebuildSession` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L669、L678、L694、L716、L734 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*service.rebuildSession` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/acp/service.go|*service.rebuildSession` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/acp/service.go:667-743` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*service.rebuildSession` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G48、G49 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/acp`。
- [ ] 运行相关包 race：`go test -race ./internal/acp`。
- [ ] 运行相关包 vet：`go vet ./internal/acp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/acp/service.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/acp/service.go|*service.rebuildSession` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-016 · 圈复杂度 · `availableCommandsFor`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-016 |
| Baseline key | `complexity\|internal/acp/service.go\|availableCommandsFor` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 29） |
| 文件 | `internal/acp/service.go` |
| 代码行 | 1165 |
| 函数/方法/包 | `availableCommandsFor` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`availableCommandsFor` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 1165-1226 行；可见控制流热点：L1166 `if ctrl == nil {`；L1176 `if desc == "" {`；L1187 `if name == "" {`；L1194 `if desc == "" {`；L1206 `if name == "" {`；L1221 `for _, cmd := range byName {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function availableCommandsFor（internal/acp/service.go:1165-1226）
- **可信生产调用方（1）：** `service::sendAvailableCommands`（`internal/acp/service.go:1151`，调用点 L1155，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `Host::Prompts`（`internal/plugin/plugin.go:116`，调用点 L1204，confidence=0.80，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `service::sessionNew`（`internal/acp/service.go:294`，调用点 L358，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/acp`、`internal/plugin`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`、`internal/acp/server_test.go`
- **需同步检查：** `internal/acp/service.go`、`internal/plugin/plugin.go`、`internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`

#### 待修改项

- [ ] 先冻结 `availableCommandsFor` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1166、L1180、L1190、L1206、L1221 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `availableCommandsFor` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/acp/service.go|availableCommandsFor` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/acp/service.go:1165-1226` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `availableCommandsFor` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G48、G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/acp`。
- [ ] 运行相关包 vet：`go vet ./internal/acp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/acp/service.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/acp/service.go|availableCommandsFor` 下降或消失且无新增/恶化。
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
