# G11 · internal/provider · anthropic 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G11 |
| 问题数量 | 10 |
| 整体优先级 | P1 |
| 涉及模块 | `internal/provider` |
| 涉及文件 | `internal/provider/anthropic/anthropic.go`<br>`internal/provider/openai/openai.go`<br>`internal/provider/retry.go`<br>`internal/provider/schema_canonicalize.go` |
| 主要治理目标 | 围绕 `internal/provider/anthropic/anthropic.go` 等治理对象降低复杂度/体量，统一处理 2 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | provider 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 6 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 11 组；无同文件组依赖 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **59/100**，对象 `*client.readStream` 的分项为严重度 18.0、集中度 16.4、影响面 12.2、可执行性 12.0；最高超限 5.00x，文件内 6 条问题，CodeGraph 记录 0 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数、嵌套深度，必须合并治理。

### 治理边界与组间关系

本组涉及 4 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-342` | `complexity\|internal/provider/anthropic/anthropic.go\|*client.readStream` | 圈复杂度 | `internal/provider/anthropic/anthropic.go` | `*client.readStream` | 308 | 50/10 | +40（5.00x） | P1 | 0 可信 + 0 待核 caller / 1 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `FN-048` | `func_lines\|internal/provider/anthropic/anthropic.go\|*client.readStream` | 函数有效行数 | `internal/provider/anthropic/anthropic.go` | `*client.readStream` | 308 | 155/80 | +75（1.94x） | P1 | 0 可信 + 0 待核 caller / 1 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `ND-017` | `nesting_depth\|internal/provider/anthropic/anthropic.go\|*client.readStream` | 嵌套深度 | `internal/provider/anthropic/anthropic.go` | `*client.readStream` | 308 | 5/4 | +1（1.25x） | P1 | 0 可信 + 0 待核 caller / 1 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-341` | `complexity\|internal/provider/anthropic/anthropic.go\|*client.buildRequest` | 圈复杂度 | `internal/provider/anthropic/anthropic.go` | `*client.buildRequest` | 191 | 30/10 | +20（3.00x） | P2 | 0 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `FN-047` | `func_lines\|internal/provider/anthropic/anthropic.go\|*client.buildRequest` | 函数有效行数 | `internal/provider/anthropic/anthropic.go` | `*client.buildRequest` | 191 | 90/80 | +10（1.13x） | P2 | 0 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `ND-016` | `nesting_depth\|internal/provider/anthropic/anthropic.go\|*client.buildRequest` | 嵌套深度 | `internal/provider/anthropic/anthropic.go` | `*client.buildRequest` | 191 | 5/4 | +1（1.25x） | P2 | 0 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-343` | `complexity\|internal/provider/openai/openai.go\|New` | 圈复杂度 | `internal/provider/openai/openai.go` | `New` | 45 | 21/10 | +11（2.10x） | P3 | 0 可信 + 0 待核 caller / 2 可信 + 2 待核 callee / 13 unresolved | 待修改 |
| `CX-348` | `complexity\|internal/provider/retry.go\|SendWithRetry` | 圈复杂度 | `internal/provider/retry.go` | `SendWithRetry` | 156 | 15/10 | +5（1.50x） | P3 | 3 可信 + 0 待核 caller / 5 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-349` | `complexity\|internal/provider/schema_canonicalize.go\|canonicalizeSchemaObject` | 圈复杂度 | `internal/provider/schema_canonicalize.go` | `canonicalizeSchemaObject` | 34 | 11/10 | +1（1.10x） | P3 | 3 可信 + 0 待核 caller / 5 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-344` | `complexity\|internal/provider/openai/openai.go\|*client.buildRequest` | 圈复杂度 | `internal/provider/openai/openai.go` | `*client.buildRequest` | 275 | 16/10 | +6（1.60x） | P3 | 0 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 8 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-342 · 圈复杂度 · `*client.readStream`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-342 |
| Baseline key | `complexity\|internal/provider/anthropic/anthropic.go\|*client.readStream` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 59） |
| 文件 | `internal/provider/anthropic/anthropic.go` |
| 代码行 | 308 |
| 函数/方法/包 | `*client.readStream` |
| Baseline / 当前复测 / 阈值 | 50 / 50 / 10 |
| 超出 | +40，5.00x |
| 同一治理对象关联问题 | `FN-048`、`ND-017` |

#### 明确的问题描述

`*client.readStream` 的圈复杂度当前为 **50**，超过限制 **10**（+40，5.00x）。定义范围为 308-480 行；可见控制流热点：L317 `if idleTimeout <= 0 { // zero-value client (constructed without New)`；L362 `for scanner.Scan() {`；L393 `if ev.ContentBlock != nil && ev.ContentBlock.Type == "tool_use" {`；L412 `if ev.Delta.Thinking != "" {`；L435 `case "message_delta":`；L468 `if !send(provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Session::Load）、状态与控制（mapStopReason）、持久化/I-O（ArchiveSink::Err）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method client::readStream（internal/provider/anthropic/anthropic.go:308-480）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `mapStopReason`（`internal/provider/anthropic/anthropic.go:498`，调用点 L474，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L455，confidence=0.70，instance-method，待源码复核）；`Session::Load`（`internal/guardian/guardian.go:261`，调用点 L458，confidence=0.70，instance-method，待源码复核）；`sendChunk`（`internal/provider/anthropic/anthropic.go:482`，调用点 L351，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/provider`、`internal/automation`、`internal/guardian`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/provider/anthropic/stall_test.go`
- **同目录测试候选：** `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`
- **需同步检查：** `internal/provider/anthropic/anthropic.go`、`internal/automation/archive_sink.go`、`internal/guardian/guardian.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`

#### 待修改项

- [ ] 先冻结 `*client.readStream` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L317、L364、L405、L429、L468 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*client.readStream` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-342、FN-048、ND-017 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/provider/anthropic/anthropic.go|*client.readStream` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/provider/anthropic/anthropic.go:308-480` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*client.readStream` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-342`、`FN-048`、`ND-017` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/provider/anthropic/stall_test.go`。
- [ ] 运行相关包测试：`go test ./internal/provider/anthropic`。
- [ ] 运行相关包 race：`go test -race ./internal/provider/anthropic`。
- [ ] 运行相关包 vet：`go vet ./internal/provider/anthropic`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/provider/anthropic/anthropic.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/provider/anthropic/anthropic.go|*client.readStream` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-048 · 函数有效行数 · `*client.readStream`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-048 |
| Baseline key | `func_lines\|internal/provider/anthropic/anthropic.go\|*client.readStream` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P2，治理对象得分 59） |
| 文件 | `internal/provider/anthropic/anthropic.go` |
| 代码行 | 308 |
| 函数/方法/包 | `*client.readStream` |
| Baseline / 当前复测 / 阈值 | 155 / 155 / 80 |
| 超出 | +75，1.94x |
| 同一治理对象关联问题 | `CX-342`、`ND-017` |

#### 明确的问题描述

`*client.readStream` 的有效代码行 **155**，超过限制 **80**（+75）。CodeGraph 定义物理范围为 308-480 行，并记录 4 个直接下游候选；候选职责分布为 配置/解析（Session::Load）、状态与控制（mapStopReason）、持久化/I-O（ArchiveSink::Err）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method client::readStream（internal/provider/anthropic/anthropic.go:308-480）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `mapStopReason`（`internal/provider/anthropic/anthropic.go:498`，调用点 L474，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L455，confidence=0.70，instance-method，待源码复核）；`Session::Load`（`internal/guardian/guardian.go:261`，调用点 L458，confidence=0.70，instance-method，待源码复核）；`sendChunk`（`internal/provider/anthropic/anthropic.go:482`，调用点 L351，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/provider`、`internal/automation`、`internal/guardian`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/provider/anthropic/stall_test.go`
- **同目录测试候选：** `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`
- **需同步检查：** `internal/provider/anthropic/anthropic.go`、`internal/automation/archive_sink.go`、`internal/guardian/guardian.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`

#### 待修改项

- [ ] 先冻结 `*client.readStream` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*client.readStream` 的 155 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L312 `Close the body if the stream stalls past c.idleTimeout so scanner.Scan()`、L314 `timer; the read loop only pings the buffered activity channel (no Timer.Reset`、L368 `SSE carries \`event:\` and \`data:\` lines; the data JSON's own \`type\` field`、L444 `Stream complete; fall through to finalize below.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-342、FN-048、ND-017 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/provider/anthropic/anthropic.go|*client.readStream` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/provider/anthropic/anthropic.go:308-480` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*client.readStream` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-342`、`FN-048`、`ND-017` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/provider/anthropic/stall_test.go`。
- [ ] 运行相关包测试：`go test ./internal/provider/anthropic`。
- [ ] 运行相关包 race：`go test -race ./internal/provider/anthropic`。
- [ ] 运行相关包 vet：`go vet ./internal/provider/anthropic`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/provider/anthropic/anthropic.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/provider/anthropic/anthropic.go|*client.readStream` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-017 · 嵌套深度 · `*client.readStream`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-017 |
| Baseline key | `nesting_depth\|internal/provider/anthropic/anthropic.go\|*client.readStream` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P3，治理对象得分 59） |
| 文件 | `internal/provider/anthropic/anthropic.go` |
| 代码行 | 308 |
| 函数/方法/包 | `*client.readStream` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-342`、`FN-048` |

#### 明确的问题描述

`*client.readStream` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L317 `if idleTimeout <= 0 { // zero-value client (constructed without New)`；L362 `for scanner.Scan() {`；L393 `if ev.ContentBlock != nil && ev.ContentBlock.Type == "tool_use" {`；L412 `if ev.Delta.Thinking != "" {`；L435 `case "message_delta":`；L468 `if !send(provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method client::readStream（internal/provider/anthropic/anthropic.go:308-480）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `mapStopReason`（`internal/provider/anthropic/anthropic.go:498`，调用点 L474，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L455，confidence=0.70，instance-method，待源码复核）；`Session::Load`（`internal/guardian/guardian.go:261`，调用点 L458，confidence=0.70，instance-method，待源码复核）；`sendChunk`（`internal/provider/anthropic/anthropic.go:482`，调用点 L351，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/provider`、`internal/automation`、`internal/guardian`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/provider/anthropic/stall_test.go`
- **同目录测试候选：** `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`
- **需同步检查：** `internal/provider/anthropic/anthropic.go`、`internal/automation/archive_sink.go`、`internal/guardian/guardian.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`

#### 待修改项

- [ ] 先冻结 `*client.readStream` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 从 L317、L364、L405、L429、L468 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-342、FN-048、ND-017 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/provider/anthropic/anthropic.go|*client.readStream` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/provider/anthropic/anthropic.go:308-480` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*client.readStream` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-342`、`FN-048`、`ND-017` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/provider/anthropic/stall_test.go`。
- [ ] 运行相关包测试：`go test ./internal/provider/anthropic`。
- [ ] 运行相关包 race：`go test -race ./internal/provider/anthropic`。
- [ ] 运行相关包 vet：`go vet ./internal/provider/anthropic`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/provider/anthropic/anthropic.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/provider/anthropic/anthropic.go|*client.readStream` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-341 · 圈复杂度 · `*client.buildRequest`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-341 |
| Baseline key | `complexity\|internal/provider/anthropic/anthropic.go\|*client.buildRequest` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 57） |
| 文件 | `internal/provider/anthropic/anthropic.go` |
| 代码行 | 191 |
| 函数/方法/包 | `*client.buildRequest` |
| Baseline / 当前复测 / 阈值 | 30 / 30 / 10 |
| 超出 | +20，3.00x |
| 同一治理对象关联问题 | `FN-047`、`ND-016` |

#### 明确的问题描述

`*client.buildRequest` 的圈复杂度当前为 **30**，超过限制 **10**（+20，3.00x）。定义范围为 191-301 行；可见控制流热点：L198 `if len(blocks) == 0 {`；L211 `if m.Content != "" {`；L220 `if mt, data, ok := provider.ParseImageDataURL(url); ok {`；L240 `if m.Content != "" {`；L268 `if n := len(system); n > 0 {`；L296 `if c.effort != "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（ParseImageDataURL）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method client::buildRequest（internal/provider/anthropic/anthropic.go:191-301）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `ephemeral`（`internal/provider/anthropic/anthropic.go:513`，调用点 L269/L271/L275，confidence=0.90，exact-match，3 条边）；`ParseImageDataURL`（`internal/provider/provider.go:61`，调用点 L220，confidence=0.90，import）；`SanitizeToolPairing`（`internal/provider/provider.go:115`，调用点 L208，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`
- **需同步检查：** `internal/provider/anthropic/anthropic.go`、`internal/provider/provider.go`、`internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`

#### 待修改项

- [ ] 先冻结 `*client.buildRequest` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L198、L214、L231、L257、L296 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*client.buildRequest` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-341、FN-047、ND-016 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/provider/anthropic/anthropic.go|*client.buildRequest` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/provider/anthropic/anthropic.go:191-301` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*client.buildRequest` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-341`、`FN-047`、`ND-016` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/provider/anthropic`。
- [ ] 运行相关包 vet：`go vet ./internal/provider/anthropic`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/provider/anthropic/anthropic.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/provider/anthropic/anthropic.go|*client.buildRequest` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-047 · 函数有效行数 · `*client.buildRequest`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-047 |
| Baseline key | `func_lines\|internal/provider/anthropic/anthropic.go\|*client.buildRequest` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 57） |
| 文件 | `internal/provider/anthropic/anthropic.go` |
| 代码行 | 191 |
| 函数/方法/包 | `*client.buildRequest` |
| Baseline / 当前复测 / 阈值 | 90 / 90 / 80 |
| 超出 | +10，1.13x |
| 同一治理对象关联问题 | `CX-341`、`ND-016` |

#### 明确的问题描述

`*client.buildRequest` 的有效代码行 **90**，超过限制 **80**（+10）。CodeGraph 定义物理范围为 191-301 行，并记录 3 个直接下游候选；候选职责分布为 配置/解析（ParseImageDataURL）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method client::buildRequest（internal/provider/anthropic/anthropic.go:191-301）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `ephemeral`（`internal/provider/anthropic/anthropic.go:513`，调用点 L269/L271/L275，confidence=0.90，exact-match，3 条边）；`ParseImageDataURL`（`internal/provider/provider.go:61`，调用点 L220，confidence=0.90，import）；`SanitizeToolPairing`（`internal/provider/provider.go:115`，调用点 L208，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`
- **需同步检查：** `internal/provider/anthropic/anthropic.go`、`internal/provider/provider.go`、`internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`

#### 待修改项

- [ ] 先冻结 `*client.buildRequest` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*client.buildRequest` 的 90 个有效行按 配置/解析 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L195 `appendBlocks adds blocks under role, merging into the previous message when`、L235 `text and its signature — reasoning without a signature (e.g. from an`、L266 `last block of the last message caches the conversation prefix, accruing hits`、L293 `text; the default omits it but still emits the signature we round-trip.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-341、FN-047、ND-016 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/provider/anthropic/anthropic.go|*client.buildRequest` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/provider/anthropic/anthropic.go:191-301` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*client.buildRequest` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-341`、`FN-047`、`ND-016` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/provider/anthropic`。
- [ ] 运行相关包 vet：`go vet ./internal/provider/anthropic`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/provider/anthropic/anthropic.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/provider/anthropic/anthropic.go|*client.buildRequest` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-016 · 嵌套深度 · `*client.buildRequest`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-016 |
| Baseline key | `nesting_depth\|internal/provider/anthropic/anthropic.go\|*client.buildRequest` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 57） |
| 文件 | `internal/provider/anthropic/anthropic.go` |
| 代码行 | 191 |
| 函数/方法/包 | `*client.buildRequest` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-341`、`FN-047` |

#### 明确的问题描述

`*client.buildRequest` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L198 `if len(blocks) == 0 {`；L211 `if m.Content != "" {`；L220 `if mt, data, ok := provider.ParseImageDataURL(url); ok {`；L240 `if m.Content != "" {`；L268 `if n := len(system); n > 0 {`；L296 `if c.effort != "" {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method client::buildRequest（internal/provider/anthropic/anthropic.go:191-301）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `ephemeral`（`internal/provider/anthropic/anthropic.go:513`，调用点 L269/L271/L275，confidence=0.90，exact-match，3 条边）；`ParseImageDataURL`（`internal/provider/provider.go:61`，调用点 L220，confidence=0.90，import）；`SanitizeToolPairing`（`internal/provider/provider.go:115`，调用点 L208，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`
- **需同步检查：** `internal/provider/anthropic/anthropic.go`、`internal/provider/provider.go`、`internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`

#### 待修改项

- [ ] 先冻结 `*client.buildRequest` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 从 L198、L214、L231、L257、L296 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-341、FN-047、ND-016 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/provider/anthropic/anthropic.go|*client.buildRequest` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/provider/anthropic/anthropic.go:191-301` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*client.buildRequest` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-341`、`FN-047`、`ND-016` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/provider/anthropic`。
- [ ] 运行相关包 vet：`go vet ./internal/provider/anthropic`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/provider/anthropic/anthropic.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/provider/anthropic/anthropic.go|*client.buildRequest` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-343 · 圈复杂度 · `New`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-343 |
| Baseline key | `complexity\|internal/provider/openai/openai.go\|New` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P1，治理对象得分 42） |
| 文件 | `internal/provider/openai/openai.go` |
| 代码行 | 45 |
| 函数/方法/包 | `New` |
| Baseline / 当前复测 / 阈值 | 21 / 21 / 10 |
| 超出 | +11，2.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`New` 的圈复杂度当前为 **21**，超过限制 **10**（+11，2.10x）。定义范围为 45-130 行；可见控制流热点：L46 `if cfg.BaseURL == "" {`；L69 `if visionDetail != "low" && visionDetail != "high" {`；L78 `switch effort {`；L85 `case minimax:`；L98 `case effort != "":`；L111 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeReasoningProtocol）、网络/协议（newHTTPClient）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function New（internal/provider/openai/openai.go:45-130）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `IsDeepSeek`（`internal/provider/openai/host.go:35`，调用点 L72，confidence=0.90，exact-match）；`IsMiniMax`（`internal/provider/openai/host.go:44`，调用点 L73，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `newHTTPClient`（`internal/provider/openai/openai.go:132`，调用点 L110，confidence=0.70，exact-match，待源码复核）；`normalizeReasoningProtocol`（`internal/provider/openai/openai.go:173`，调用点 L65，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 13 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/provider/openai/openai_test.go`
- **同目录测试候选：** `internal/provider/openai/effort_test.go`、`internal/provider/openai/fetch_models_test.go`、`internal/provider/openai/host_test.go`、`internal/provider/openai/image_test.go`、`internal/provider/openai/realcache_test.go`、`internal/provider/openai/reconnect_test.go`、`internal/provider/openai/stall_test.go`、`internal/provider/openai/think_test.go`
- **需同步检查：** `internal/provider/openai/openai.go`、`internal/provider/openai/host.go`、`internal/provider/openai/openai_test.go`、`internal/provider/openai/effort_test.go`、`internal/provider/openai/fetch_models_test.go`、`internal/provider/openai/host_test.go`

#### 待修改项

- [ ] 先冻结 `New` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L46、L74、L81、L94、L111 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `New` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/provider/openai/openai.go|New` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/provider/openai/openai.go:45-130` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `New` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/provider/openai/openai_test.go`。
- [ ] 运行相关包测试：`go test ./internal/provider/openai`。
- [ ] 运行相关包 vet：`go vet ./internal/provider/openai`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/provider/openai/openai.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/provider/openai/openai.go|New` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-348 · 圈复杂度 · `SendWithRetry`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-348 |
| Baseline key | `complexity\|internal/provider/retry.go\|SendWithRetry` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 42） |
| 文件 | `internal/provider/retry.go` |
| 代码行 | 156 |
| 函数/方法/包 | `SendWithRetry` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`SendWithRetry` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 156-213 行；可见控制流热点：L162 `for attempt := 0; attempt <= MaxRetries; attempt++ {`；L165 `if notify != nil {`；L171 `case <-time.After(delay):`；L181 `if err != nil {`；L197 `if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {`；L207 `if !RetryableStatus(resp.StatusCode) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseRetryAfter）、持久化/I-O（ArchiveSink::Err）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function SendWithRetry（internal/provider/retry.go:156-213）
- **可信生产调用方（3）：** `client::Stream`（`internal/provider/anthropic/anthropic.go:153`，调用点 L175，confidence=0.90，import）；`client::Stream`（`internal/provider/openai/openai.go:190`，调用点 L213，confidence=0.90，import）；`client::streamWithReconnect`（`internal/provider/openai/openai.go:233`，调用点 L252，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `retryNotifyFromContext`（`internal/provider/retry.go:62`，调用点 L157，confidence=0.90，exact-match）；`RetryableStatus`（`internal/provider/retry.go:86`，调用点 L207，confidence=0.90，exact-match）；`transientErr`（`internal/provider/retry.go:90`，调用点 L182，confidence=0.90，exact-match）；`backoffDelay`（`internal/provider/retry.go:121`，调用点 L164，confidence=0.90，exact-match）；`parseRetryAfter`（`internal/provider/retry.go:135`，调用点 L193，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L170，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `client::Stream`（`internal/provider/anthropic/anthropic.go:153`，调用点 L175，confidence=0.90，import，待源码复核）；`client::Stream`（`internal/provider/openai/openai.go:190`，调用点 L213，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/provider`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/provider/retry_test.go`
- **同目录测试候选：** `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/image_test.go`、`internal/provider/openai/effort_test.go`、`internal/provider/openai/fetch_models_test.go`、`internal/provider/openai/host_test.go`、`internal/provider/openai/image_test.go`、`internal/provider/openai/openai_test.go`、`internal/provider/openai/realcache_test.go`、`internal/provider/openai/reconnect_test.go`、`internal/provider/openai/stall_test.go`
- **需同步检查：** `internal/provider/retry.go`、`internal/provider/anthropic/anthropic.go`、`internal/provider/openai/openai.go`、`internal/provider/retry_test.go`、`internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`

#### 待修改项

- [ ] 先冻结 `SendWithRetry` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L162、L168、L177、L188、L207 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `SendWithRetry` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/provider/retry.go|SendWithRetry` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/provider/retry.go:156-213` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `SendWithRetry` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/provider/retry_test.go`。
- [ ] 运行相关包测试：`go test ./internal/provider`。
- [ ] 运行相关包 race：`go test -race ./internal/provider`。
- [ ] 运行相关包 vet：`go vet ./internal/provider`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/provider/retry.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/provider/retry.go|SendWithRetry` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-349 · 圈复杂度 · `canonicalizeSchemaObject`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-349 |
| Baseline key | `complexity\|internal/provider/schema_canonicalize.go\|canonicalizeSchemaObject` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/provider/schema_canonicalize.go` |
| 代码行 | 34 |
| 函数/方法/包 | `canonicalizeSchemaObject` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`canonicalizeSchemaObject` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 34-70 行；可见控制流热点：L35 `switch val := v.(type) {`；L37 `for k, inner := range val {`；L39 `case "properties", "patternProperties", "$defs", "definitions", "dependentSchemas":`；L47 `if req, ok := val["required"]; ok {`；L58 `if dr, ok := val["dependentRequired"]; ok && !isJSONObject(dr) {`；L63 `for i, elem := range val {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function canonicalizeSchemaObject（internal/provider/schema_canonicalize.go:34-70）
- **可信生产调用方（3）：** `canonicalizeSchemaObject`（`internal/provider/schema_canonicalize.go:34`，调用点 L44/L64，confidence=0.90，exact-match，2 条边）；`canonicalizeNamedSchemas`（`internal/provider/schema_canonicalize.go:72`，调用点 L75/L78，confidence=0.90，exact-match，2 条边）；`canonicalizeSchemaValue`（`internal/provider/schema_canonicalize.go:30`，调用点 L31，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `canonicalizeSchemaObject`（`internal/provider/schema_canonicalize.go:34`，调用点 L44/L64，confidence=0.90，exact-match，2 条边）；`canonicalizeNamedSchemas`（`internal/provider/schema_canonicalize.go:72`，调用点 L40，confidence=0.90，exact-match）；`canonicalizeDependentRequired`（`internal/provider/schema_canonicalize.go:83`，调用点 L42，confidence=0.90，exact-match）；`isJSONObject`（`internal/provider/schema_canonicalize.go:98`，调用点 L58，confidence=0.90，exact-match）；`sortSchemaArray`（`internal/provider/schema_canonicalize.go:103`，调用点 L49，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/image_test.go`、`internal/provider/openai/effort_test.go`、`internal/provider/openai/fetch_models_test.go`、`internal/provider/openai/host_test.go`、`internal/provider/openai/image_test.go`、`internal/provider/openai/openai_test.go`、`internal/provider/openai/realcache_test.go`、`internal/provider/openai/reconnect_test.go`、`internal/provider/openai/stall_test.go`
- **需同步检查：** `internal/provider/schema_canonicalize.go`、`internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/image_test.go`

#### 待修改项

- [ ] 先冻结 `canonicalizeSchemaObject` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L35、L38、L41、L58、L63 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `canonicalizeSchemaObject` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/provider/schema_canonicalize.go|canonicalizeSchemaObject` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/provider/schema_canonicalize.go:34-70` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `canonicalizeSchemaObject` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/provider/anthropic/anthropic_test.go`、`internal/provider/anthropic/image_test.go`、`internal/provider/anthropic/stall_test.go`、`internal/provider/image_test.go`、`internal/provider/openai/effort_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/provider`。
- [ ] 运行相关包 vet：`go vet ./internal/provider`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/provider/schema_canonicalize.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/provider/schema_canonicalize.go|canonicalizeSchemaObject` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-344 · 圈复杂度 · `*client.buildRequest`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-344 |
| Baseline key | `complexity\|internal/provider/openai/openai.go\|*client.buildRequest` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 38） |
| 文件 | `internal/provider/openai/openai.go` |
| 代码行 | 275 |
| 函数/方法/包 | `*client.buildRequest` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*client.buildRequest` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 275-343 行；可见控制流热点：L281 `for i, m := range src {`；L293 `for _, tc := range m.ToolCalls {`；L300 `case c.vision && m.Role == provider.RoleUser && len(m.Images) > 0:`；L309 `for _, t := range req.Tools {`；L327 `case c.deepseek:`；L336 `if t == "" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method client::buildRequest（internal/provider/openai/openai.go:275-343）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `imageContentParts`（`internal/provider/openai/openai.go:717`，调用点 L301，confidence=0.90，exact-match）；`SanitizeToolPairing`（`internal/provider/provider.go:115`，调用点 L279，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 8 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/provider/openai/openai_test.go`
- **同目录测试候选：** `internal/provider/openai/effort_test.go`、`internal/provider/openai/fetch_models_test.go`、`internal/provider/openai/host_test.go`、`internal/provider/openai/image_test.go`、`internal/provider/openai/realcache_test.go`、`internal/provider/openai/reconnect_test.go`、`internal/provider/openai/stall_test.go`、`internal/provider/openai/think_test.go`
- **需同步检查：** `internal/provider/openai/openai.go`、`internal/provider/provider.go`、`internal/provider/openai/openai_test.go`、`internal/provider/openai/effort_test.go`、`internal/provider/openai/fetch_models_test.go`、`internal/provider/openai/host_test.go`

#### 待修改项

- [ ] 先冻结 `*client.buildRequest` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L281、L299、L302、L327、L336 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `*client.buildRequest` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/provider/openai/openai.go|*client.buildRequest` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/provider/openai/openai.go:275-343` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*client.buildRequest` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/provider/openai/openai_test.go`。
- [ ] 运行相关包测试：`go test ./internal/provider/openai`。
- [ ] 运行相关包 vet：`go vet ./internal/provider/openai`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/provider/openai/openai.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/provider/openai/openai.go|*client.buildRequest` 下降或消失且无新增/恶化。
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
