# G13 · internal/plugin · lazy 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G13 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/plugin`、`internal/tool` |
| 涉及文件 | `internal/plugin/lazy.go`<br>`internal/plugin/plugin.go`<br>`internal/tool/builtin/codeindex.go`<br>`internal/plugin/transport_stdio.go`<br>`internal/plugin/transport_http.go` |
| 主要治理目标 | 围绕 `internal/tool/builtin/codeindex.go` 等治理对象降低复杂度/体量，统一处理 3 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | plugin / tool 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 低：仍需开始前复查开放 PR |
| 冲突依据 | 未发现静态热点或本地远端引用重叠 |
| 建议处理顺序 | 总序第 13 组；先于 G32 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **55/100**，对象 `*lazyTool.Execute` 的分项为严重度 10.0、集中度 14.6、影响面 10.3、可执行性 20.0；最高超限 1.90x，文件内 3 条问题，CodeGraph 记录 0 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数、嵌套深度，必须合并治理。

### 治理边界与组间关系

本组涉及 5 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G32，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-325` | `complexity\|internal/plugin/lazy.go\|*lazyTool.Execute` | 圈复杂度 | `internal/plugin/lazy.go` | `*lazyTool.Execute` | 203 | 19/10 | +9（1.90x） | P2 | 0 可信 + 0 待核 caller / 6 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `FN-045` | `func_lines\|internal/plugin/lazy.go\|*lazyTool.Execute` | 函数有效行数 | `internal/plugin/lazy.go` | `*lazyTool.Execute` | 203 | 93/80 | +13（1.16x） | P2 | 0 可信 + 0 待核 caller / 6 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `ND-015` | `nesting_depth\|internal/plugin/lazy.go\|*lazyTool.Execute` | 嵌套深度 | `internal/plugin/lazy.go` | `*lazyTool.Execute` | 203 | 5/4 | +1（1.25x） | P2 | 0 可信 + 0 待核 caller / 6 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-326` | `complexity\|internal/plugin/plugin.go\|Start` | 圈复杂度 | `internal/plugin/plugin.go` | `Start` | 241 | 17/10 | +7（1.70x） | P2 | 0 可信 + 2 待核 caller / 5 可信 + 5 待核 callee / 0 unresolved | 待修改 |
| `FN-046` | `func_lines\|internal/plugin/plugin.go\|Start` | 函数有效行数 | `internal/plugin/plugin.go` | `Start` | 241 | 100/80 | +20（1.25x） | P2 | 0 可信 + 2 待核 caller / 5 可信 + 5 待核 callee / 0 unresolved | 待修改 |
| `CX-377` | `complexity\|internal/tool/builtin/codeindex.go\|codeIndex.parseGo` | 圈复杂度 | `internal/tool/builtin/codeindex.go` | `codeIndex.parseGo` | 191 | 14/10 | +4（1.40x） | P2 | 0 可信 + 1 待核 caller / 2 可信 + 1 待核 callee / 9 unresolved | 待修改 |
| `ND-018` | `nesting_depth\|internal/tool/builtin/codeindex.go\|codeIndex.parseGo` | 嵌套深度 | `internal/tool/builtin/codeindex.go` | `codeIndex.parseGo` | 191 | 5/4 | +1（1.25x） | P2 | 0 可信 + 1 待核 caller / 2 可信 + 1 待核 callee / 9 unresolved | 待修改 |
| `FL-017` | `file_lines\|internal/plugin/plugin.go\|` | 文件有效行数 | `internal/plugin/plugin.go` | `internal/plugin/plugin.go` | 1 | 859/800 | +59（1.07x） | P3 | 5 可信 + 2 待核 caller / 8 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `CX-328` | `complexity\|internal/plugin/transport_stdio.go\|newStdioTransport` | 圈复杂度 | `internal/plugin/transport_stdio.go` | `newStdioTransport` | 57 | 13/10 | +3（1.30x） | P3 | 0 可信 + 0 待核 caller / 8 可信 + 0 待核 callee / 11 unresolved | 待修改 |
| `CX-327` | `complexity\|internal/plugin/transport_http.go\|*httpTransport.readSSEResponse` | 圈复杂度 | `internal/plugin/transport_http.go` | `*httpTransport.readSSEResponse` | 170 | 14/10 | +4（1.40x） | P3 | 0 可信 + 1 待核 caller / 0 可信 + 2 待核 callee / 16 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-325 · 圈复杂度 · `*lazyTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-325 |
| Baseline key | `complexity\|internal/plugin/lazy.go\|*lazyTool.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 55） |
| 文件 | `internal/plugin/lazy.go` |
| 代码行 | 203 |
| 函数/方法/包 | `*lazyTool.Execute` |
| Baseline / 当前复测 / 阈值 | 19 / 19 / 10 |
| 超出 | +9，1.90x |
| 同一治理对象关联问题 | `FN-045`、`ND-015` |

#### 明确的问题描述

`*lazyTool.Execute` 的圈复杂度当前为 **19**，超过限制 **10**（+9，1.90x）。定义范围为 203-326 行；可见控制流热点：L208 `if sp.state == spawnReady && !sp.swapped {`；L216 `if real == nil {`；L231 `if !lt.hasCache {`；L262 `if errors.Is(err, context.DeadlineExceeded) {`；L284 `for _, t := range tools {`；L318 `if r == nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（IsServerAlreadyConnected）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method lazyTool::Execute（internal/plugin/lazy.go:203-326）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `IsServerAlreadyConnected`（`internal/plugin/plugin.go:202`，调用点 L279，confidence=0.90，exact-match）；`Host::RecordFailure`（`internal/plugin/plugin.go:585`，调用点 L308，confidence=0.90，exact-match）；`Host::beginDeferredSpawn`（`internal/plugin/plugin.go:636`，调用点 L236，confidence=0.90，exact-match）；`Host::endDeferredSpawn`（`internal/plugin/plugin.go:646`，调用点 L244，confidence=0.90，exact-match）；`Host::ToolsFor`（`internal/plugin/plugin.go:712`，调用点 L282，confidence=0.90，exact-match）；`Host::AddWithLifecycle`（`internal/plugin/plugin.go:777`，调用点 L257，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `lazySpawn::trySwap`（`internal/plugin/lazy.go:153`，调用点 L209/L288/L316，confidence=0.70，instance-method，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/plugin`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`、`internal/plugin/hotadd_test.go`、`internal/plugin/lazy_test.go`、`internal/plugin/main_test.go`、`internal/plugin/planmode_untrusted_test.go`、`internal/plugin/plugin_test.go`、`internal/plugin/stats_test.go`、`internal/plugin/stdio_cancel_test.go`
- **需同步检查：** `internal/plugin/lazy.go`、`internal/plugin/plugin.go`、`internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`

#### 待修改项

- [ ] 逐段核对 L208、L221、L261、L282、L318 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*lazyTool.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-325、FN-045、ND-015 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/plugin/lazy.go|*lazyTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/plugin/lazy.go:203-326` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*lazyTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-325`、`FN-045`、`ND-015` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/plugin`。
- [ ] 运行相关包 race：`go test -race ./internal/plugin`。
- [ ] 运行相关包 vet：`go vet ./internal/plugin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/plugin/lazy.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/plugin/lazy.go|*lazyTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-045 · 函数有效行数 · `*lazyTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-045 |
| Baseline key | `func_lines\|internal/plugin/lazy.go\|*lazyTool.Execute` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 55） |
| 文件 | `internal/plugin/lazy.go` |
| 代码行 | 203 |
| 函数/方法/包 | `*lazyTool.Execute` |
| Baseline / 当前复测 / 阈值 | 93 / 93 / 80 |
| 超出 | +13，1.16x |
| 同一治理对象关联问题 | `CX-325`、`ND-015` |

#### 明确的问题描述

`*lazyTool.Execute` 的有效代码行 **93**，超过限制 **80**（+13）。CodeGraph 定义物理范围为 203-326 行，并记录 7 个直接下游候选；候选职责分布为 持久化/I-O（IsServerAlreadyConnected）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method lazyTool::Execute（internal/plugin/lazy.go:203-326）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `IsServerAlreadyConnected`（`internal/plugin/plugin.go:202`，调用点 L279，confidence=0.90，exact-match）；`Host::RecordFailure`（`internal/plugin/plugin.go:585`，调用点 L308，confidence=0.90，exact-match）；`Host::beginDeferredSpawn`（`internal/plugin/plugin.go:636`，调用点 L236，confidence=0.90，exact-match）；`Host::endDeferredSpawn`（`internal/plugin/plugin.go:646`，调用点 L244，confidence=0.90，exact-match）；`Host::ToolsFor`（`internal/plugin/plugin.go:712`，调用点 L282，confidence=0.90，exact-match）；`Host::AddWithLifecycle`（`internal/plugin/plugin.go:777`，调用点 L257，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `lazySpawn::trySwap`（`internal/plugin/lazy.go:153`，调用点 L209/L288/L316，confidence=0.70，instance-method，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/plugin`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`、`internal/plugin/hotadd_test.go`、`internal/plugin/lazy_test.go`、`internal/plugin/main_test.go`、`internal/plugin/planmode_untrusted_test.go`、`internal/plugin/plugin_test.go`、`internal/plugin/stats_test.go`、`internal/plugin/stdio_cancel_test.go`
- **需同步检查：** `internal/plugin/lazy.go`、`internal/plugin/plugin.go`、`internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`

#### 待修改项

- [ ] 把 `*lazyTool.Execute` 的 93 个有效行按 持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L207 `Catch up on a background spawn that finished while we were idle.`、L252 `unreachable MCP server can't hang the whole turn indefinitely`、L274 `connected client once the other spawn finishes.`、L301 `ToolsFor failed — not our fault, don't record as failure.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-325、FN-045、ND-015 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/plugin/lazy.go|*lazyTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/plugin/lazy.go:203-326` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*lazyTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-325`、`FN-045`、`ND-015` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/plugin`。
- [ ] 运行相关包 race：`go test -race ./internal/plugin`。
- [ ] 运行相关包 vet：`go vet ./internal/plugin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/plugin/lazy.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/plugin/lazy.go|*lazyTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-015 · 嵌套深度 · `*lazyTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-015 |
| Baseline key | `nesting_depth\|internal/plugin/lazy.go\|*lazyTool.Execute` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 55） |
| 文件 | `internal/plugin/lazy.go` |
| 代码行 | 203 |
| 函数/方法/包 | `*lazyTool.Execute` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-325`、`FN-045` |

#### 明确的问题描述

`*lazyTool.Execute` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L208 `if sp.state == spawnReady && !sp.swapped {`；L216 `if real == nil {`；L231 `if !lt.hasCache {`；L262 `if errors.Is(err, context.DeadlineExceeded) {`；L284 `for _, t := range tools {`；L318 `if r == nil {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method lazyTool::Execute（internal/plugin/lazy.go:203-326）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `IsServerAlreadyConnected`（`internal/plugin/plugin.go:202`，调用点 L279，confidence=0.90，exact-match）；`Host::RecordFailure`（`internal/plugin/plugin.go:585`，调用点 L308，confidence=0.90，exact-match）；`Host::beginDeferredSpawn`（`internal/plugin/plugin.go:636`，调用点 L236，confidence=0.90，exact-match）；`Host::endDeferredSpawn`（`internal/plugin/plugin.go:646`，调用点 L244，confidence=0.90，exact-match）；`Host::ToolsFor`（`internal/plugin/plugin.go:712`，调用点 L282，confidence=0.90，exact-match）；`Host::AddWithLifecycle`（`internal/plugin/plugin.go:777`，调用点 L257，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `lazySpawn::trySwap`（`internal/plugin/lazy.go:153`，调用点 L209/L288/L316，confidence=0.70，instance-method，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/plugin`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`、`internal/plugin/hotadd_test.go`、`internal/plugin/lazy_test.go`、`internal/plugin/main_test.go`、`internal/plugin/planmode_untrusted_test.go`、`internal/plugin/plugin_test.go`、`internal/plugin/stats_test.go`、`internal/plugin/stdio_cancel_test.go`
- **需同步检查：** `internal/plugin/lazy.go`、`internal/plugin/plugin.go`、`internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`

#### 待修改项

- [ ] 从 L208、L221、L261、L282、L318 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-325、FN-045、ND-015 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/plugin/lazy.go|*lazyTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/plugin/lazy.go:203-326` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*lazyTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-325`、`FN-045`、`ND-015` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/plugin`。
- [ ] 运行相关包 race：`go test -race ./internal/plugin`。
- [ ] 运行相关包 vet：`go vet ./internal/plugin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/plugin/lazy.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/plugin/lazy.go|*lazyTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-326 · 圈复杂度 · `Start`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-326 |
| Baseline key | `complexity\|internal/plugin/plugin.go\|Start` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 48） |
| 文件 | `internal/plugin/plugin.go` |
| 代码行 | 241 |
| 函数/方法/包 | `Start` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | `FN-046` |

#### 明确的问题描述

`Start` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 241-371 行；可见控制流热点：L242 `if len(specs) == 0 {`；L268 `for i, s := range specs {`；L294 `if err != nil {`；L344 `for range specs {`；L353 `if p.AbortOnError {`；L366 `if firstErr != nil {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（RecordStartup、start）、持久化/I-O（SaveCachedSchema、ArchiveSink::Err）、渲染/输出（SpecFingerprint）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function Start（internal/plugin/plugin.go:241-371）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `StartAll`（`internal/plugin/plugin.go:212`，调用点 L213，confidence=0.70，exact-match，待源码复核）；`StartAvailable`（`internal/plugin/plugin.go:222`，调用点 L223，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（5）：** `Done`（`internal/jobs/jobs.go:37`，调用点 L298/L308/L322，confidence=0.90，exact-match，3 条边）；`RecordStartup`（`internal/plugin/stats.go:69`，调用点 L298/L308/L323，confidence=0.90，exact-match，3 条边）；`cacheableToolsOf`（`internal/plugin/cache.go:31`，调用点 L330，confidence=0.90，exact-match）；`SpecFingerprint`（`internal/plugin/cache.go:81`，调用点 L325，confidence=0.90，exact-match）；`SaveCachedSchema`（`internal/plugin/cache.go:131`，调用点 L324，confidence=0.90，exact-match）
- **待源码复核被调候选（5）：** `Host::Add`（`internal/plugin/plugin.go:745`，调用点 L297/L307/L320，confidence=0.70，exact-match，3 条边，待源码复核）；`ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L284，confidence=0.70，instance-method，待源码复核）；`Host::RecordFailure`（`internal/plugin/plugin.go:585`，调用点 L358，confidence=0.70，instance-method，待源码复核）；`start`（`internal/plugin/plugin.go:906`，调用点 L293，confidence=0.70，exact-match，待源码复核）；`Client::listTools`（`internal/plugin/plugin.go:1016`，调用点 L303，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `StartAll`（`internal/plugin/plugin.go:212`，调用点 L213，confidence=0.70，exact-match，待源码复核）；`StartAvailable`（`internal/plugin/plugin.go:222`，调用点 L223，confidence=0.70，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/plugin`、`internal/jobs`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/plugin/plugin_test.go`
- **同目录测试候选：** `internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`、`internal/plugin/hotadd_test.go`、`internal/plugin/lazy_test.go`、`internal/plugin/main_test.go`、`internal/plugin/planmode_untrusted_test.go`、`internal/plugin/stats_test.go`、`internal/plugin/stdio_cancel_test.go`
- **需同步检查：** `internal/plugin/plugin.go`、`internal/jobs/jobs.go`、`internal/plugin/stats.go`、`internal/plugin/cache.go`、`internal/plugin/plugin_test.go`、`internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`

#### 待修改项

- [ ] 逐段核对 L242、L275、L304、L352、L366 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Start` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-326、FN-046 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/plugin/plugin.go|Start` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/plugin/plugin.go:241-371` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Start` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-326`、`FN-046` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/plugin/plugin_test.go`。
- [ ] 运行相关包测试：`go test ./internal/plugin`。
- [ ] 运行相关包 race：`go test -race ./internal/plugin`。
- [ ] 运行相关包 vet：`go vet ./internal/plugin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/plugin/plugin.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/plugin/plugin.go|Start` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-046 · 函数有效行数 · `Start`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-046 |
| Baseline key | `func_lines\|internal/plugin/plugin.go\|Start` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 48） |
| 文件 | `internal/plugin/plugin.go` |
| 代码行 | 241 |
| 函数/方法/包 | `Start` |
| Baseline / 当前复测 / 阈值 | 100 / 100 / 80 |
| 超出 | +20，1.25x |
| 同一治理对象关联问题 | `CX-326` |

#### 明确的问题描述

`Start` 的有效代码行 **100**，超过限制 **80**（+20）。CodeGraph 定义物理范围为 241-371 行，并记录 10 个直接下游候选；候选职责分布为 状态与控制（RecordStartup、start）、持久化/I-O（SaveCachedSchema、ArchiveSink::Err）、渲染/输出（SpecFingerprint）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function Start（internal/plugin/plugin.go:241-371）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `StartAll`（`internal/plugin/plugin.go:212`，调用点 L213，confidence=0.70，exact-match，待源码复核）；`StartAvailable`（`internal/plugin/plugin.go:222`，调用点 L223，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（5）：** `Done`（`internal/jobs/jobs.go:37`，调用点 L298/L308/L322，confidence=0.90，exact-match，3 条边）；`RecordStartup`（`internal/plugin/stats.go:69`，调用点 L298/L308/L323，confidence=0.90，exact-match，3 条边）；`cacheableToolsOf`（`internal/plugin/cache.go:31`，调用点 L330，confidence=0.90，exact-match）；`SpecFingerprint`（`internal/plugin/cache.go:81`，调用点 L325，confidence=0.90，exact-match）；`SaveCachedSchema`（`internal/plugin/cache.go:131`，调用点 L324，confidence=0.90，exact-match）
- **待源码复核被调候选（5）：** `Host::Add`（`internal/plugin/plugin.go:745`，调用点 L297/L307/L320，confidence=0.70，exact-match，3 条边，待源码复核）；`ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L284，confidence=0.70，instance-method，待源码复核）；`Host::RecordFailure`（`internal/plugin/plugin.go:585`，调用点 L358，confidence=0.70，instance-method，待源码复核）；`start`（`internal/plugin/plugin.go:906`，调用点 L293，confidence=0.70，exact-match，待源码复核）；`Client::listTools`（`internal/plugin/plugin.go:1016`，调用点 L303，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `StartAll`（`internal/plugin/plugin.go:212`，调用点 L213，confidence=0.70，exact-match，待源码复核）；`StartAvailable`（`internal/plugin/plugin.go:222`，调用点 L223，confidence=0.70，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/plugin`、`internal/jobs`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/plugin/plugin_test.go`
- **同目录测试候选：** `internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`、`internal/plugin/hotadd_test.go`、`internal/plugin/lazy_test.go`、`internal/plugin/main_test.go`、`internal/plugin/planmode_untrusted_test.go`、`internal/plugin/stats_test.go`、`internal/plugin/stdio_cancel_test.go`
- **需同步检查：** `internal/plugin/plugin.go`、`internal/jobs/jobs.go`、`internal/plugin/stats.go`、`internal/plugin/cache.go`、`internal/plugin/plugin_test.go`、`internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`

#### 待修改项

- [ ] 把 `Start` 的 100 个有效行按 状态与控制、持久化/I-O、渲染/输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L254 `A buffered channel acts as a counting semaphore. Capacity 0/negative`、L290 `Transport on the parent ctx, startup RPCs on the timed callCtx: the`、L334 `Prompts and resources are deferred to StartPhaseB so the boot path`、L364 `prompts/resources are filled in later by StartPhaseB.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-326、FN-046 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/plugin/plugin.go|Start` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/plugin/plugin.go:241-371` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Start` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-326`、`FN-046` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/plugin/plugin_test.go`。
- [ ] 运行相关包测试：`go test ./internal/plugin`。
- [ ] 运行相关包 race：`go test -race ./internal/plugin`。
- [ ] 运行相关包 vet：`go vet ./internal/plugin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/plugin/plugin.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/plugin/plugin.go|Start` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-377 · 圈复杂度 · `codeIndex.parseGo`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-377 |
| Baseline key | `complexity\|internal/tool/builtin/codeindex.go\|codeIndex.parseGo` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 46） |
| 文件 | `internal/tool/builtin/codeindex.go` |
| 代码行 | 191 |
| 函数/方法/包 | `codeIndex.parseGo` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | `ND-018` |

#### 明确的问题描述

`codeIndex.parseGo` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 191-243 行；可见控制流热点：L194 `if err != nil {`；L212 `switch d := decl.(type) {`；L221 `case *ast.GenDecl:`；L223 `switch s := spec.(type) {`；L227 `case *ast.StructType:`；L235 `for _, name := range s.Names {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method codeIndex::parseGo（internal/tool/builtin/codeindex.go:191-243）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `codeIndex::parseFile`（`internal/tool/builtin/codeindex.go:178`，调用点 L186，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `exprName`（`internal/tool/builtin/codeindex.go:345`，调用点 L217，confidence=0.90，exact-match）；`goFuncSignature`（`internal/tool/builtin/codeindex.go:358`，调用点 L220，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `codeIndex::displayPath`（`internal/tool/builtin/codeindex.go:273`，调用点 L205，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 9 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/codeindex.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L194、L213、L222、L227、L235 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `codeIndex.parseGo` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-377、ND-018 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/codeindex.go|codeIndex.parseGo` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/codeindex.go:191-243` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `codeIndex.parseGo` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-377`、`ND-018` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/codeindex.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/codeindex.go|codeIndex.parseGo` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-018 · 嵌套深度 · `codeIndex.parseGo`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-018 |
| Baseline key | `nesting_depth\|internal/tool/builtin/codeindex.go\|codeIndex.parseGo` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 46） |
| 文件 | `internal/tool/builtin/codeindex.go` |
| 代码行 | 191 |
| 函数/方法/包 | `codeIndex.parseGo` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-377` |

#### 明确的问题描述

`codeIndex.parseGo` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L194 `if err != nil {`；L212 `switch d := decl.(type) {`；L221 `case *ast.GenDecl:`；L223 `switch s := spec.(type) {`；L227 `case *ast.StructType:`；L235 `for _, name := range s.Names {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method codeIndex::parseGo（internal/tool/builtin/codeindex.go:191-243）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `codeIndex::parseFile`（`internal/tool/builtin/codeindex.go:178`，调用点 L186，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `exprName`（`internal/tool/builtin/codeindex.go:345`，调用点 L217，confidence=0.90，exact-match）；`goFuncSignature`（`internal/tool/builtin/codeindex.go:358`，调用点 L220，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `codeIndex::displayPath`（`internal/tool/builtin/codeindex.go:273`，调用点 L205，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 9 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/codeindex.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 从 L194、L213、L222、L227、L235 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-377、ND-018 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/tool/builtin/codeindex.go|codeIndex.parseGo` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/codeindex.go:191-243` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `codeIndex.parseGo` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-377`、`ND-018` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/codeindex.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/tool/builtin/codeindex.go|codeIndex.parseGo` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FL-017 · 文件有效行数 · `internal/plugin/plugin.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-017 |
| Baseline key | `file_lines\|internal/plugin/plugin.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 43） |
| 文件 | `internal/plugin/plugin.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/plugin/plugin.go` |
| Baseline / 当前复测 / 阈值 | 859 / 859 / 800 |
| 超出 | +59，1.07x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/plugin/plugin.go` 当前有效代码行 **859**，超过限制 **800**（+59）。CodeGraph 在文件中索引到的主要大符号包括 Start@241-371、Host::addConnectedWithLifecycle@812-855、Client::listTools@1016-1054、Host::Remove@860-897、Host::AddWithLifecycle@777-806、Host::fetchPrompts@420-447；文件同时承载的调用影响覆盖 14 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 63 个函数/方法节点。
- **可信生产调用方（5）：** `chatTUI::showMCPStatus`（`internal/cli/chat_tui.go:3567`，调用点 L3568/L3572，confidence=0.90，exact-match，6 条边）；`lazyTool::Execute`（`internal/plugin/lazy.go:203`，调用点 L257/L308/L282/L236，confidence=0.90，exact-match，6 条边）；`LazyToolset`（`internal/plugin/lazy.go:343`，调用点 L345/L375/L369/L372，confidence=0.90，instance-method，5 条边）；`lazySpawn::run`（`internal/plugin/lazy.go:90`，调用点 L91/L128/L107/L103，confidence=0.90，exact-match，4 条边）；`chatTUI::buildMCPSnapshot`（`internal/cli/mcp_manager.go:298`，调用点 L315/L338/L327，confidence=0.90，exact-match，3 条边）
- **待源码复核调用方候选（2）：** `Build`（`internal/boot/boot.go:152`，调用点 L899/L909/L450/L469，confidence=0.80，instance-method，10 条边，待源码复核）；`App::mcpServersView`（`desktop/app.go:4692`，调用点 L4728/L4765/L4755，confidence=0.70，instance-method，3 条边，待源码复核）
- **可信被调用函数（8）：** `Done`（`internal/jobs/jobs.go:37`，调用点 L298/L308/L322/L647，confidence=0.90，exact-match，4 条边）；`RecordStartup`（`internal/plugin/stats.go:69`，调用点 L298/L308/L323，confidence=0.90，exact-match，3 条边）；`Manager::Wait`（`internal/jobs/jobs.go:702`，调用点 L387/L395，confidence=0.90，exact-match，2 条边）；`canonicalizeSchema`（`internal/plugin/canonicalize.go:24`，调用点 L1046/L1155，confidence=0.90，exact-match，2 条边）；`SaveCachedSchema`（`internal/plugin/cache.go:131`，调用点 L324，confidence=0.90，exact-match）；`SpecFingerprint`（`internal/plugin/cache.go:81`，调用点 L325，confidence=0.90，exact-match）；`cacheableToolsOf`（`internal/plugin/cache.go:31`，调用点 L330，confidence=0.90，exact-match）；`sortToolsByName`（`internal/plugin/canonicalize.go:12`，调用点 L1053，confidence=0.90，exact-match）
- **待源码复核被调候选（4）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L284/L790/L792/L758，confidence=0.70，instance-method，4 条边，待源码复核）；`Sink::Emit`（`internal/event/event.go:362`，调用点 L442/L468，confidence=0.80，instance-method，2 条边，待源码复核）；`Client::listPrompts`（`internal/plugin/prompts.go:36`，调用点 L429，confidence=0.70，instance-method，待源码复核）；`Client::listResources`（`internal/plugin/resources.go:21`，调用点 L458，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 14 个文件；`internal/plugin`、`internal/boot`、`internal/cli`、`desktop`、`internal/automation`、`internal/jobs`、`internal/event`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`、`internal/plugin/hotadd_test.go`、`internal/acp/server_test.go`、`internal/plugin/plugin_test.go`、`desktop/app_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/lazy_test.go`
- **同目录测试候选：** `internal/plugin/cache_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/main_test.go`、`internal/plugin/planmode_untrusted_test.go`、`internal/plugin/stats_test.go`、`internal/plugin/stdio_cancel_test.go`
- **需同步检查：** `internal/plugin/plugin.go`、`internal/boot/boot.go`、`internal/cli/chat_tui.go`、`internal/plugin/lazy.go`、`internal/automation/archive_sink.go`、`internal/jobs/jobs.go`、`internal/plugin/stats.go`、`internal/event/event.go`、`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`、`internal/plugin/hotadd_test.go`、`internal/acp/server_test.go`

#### 待修改项

- [ ] 围绕主要符号 `Start`、`Host::addConnectedWithLifecycle`、`Client::listTools`、`Host::Remove`、`Host::AddWithLifecycle` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-326、FN-046。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/plugin/plugin.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/plugin/plugin.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/plugin/plugin.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`、`internal/plugin/hotadd_test.go`、`internal/acp/server_test.go`、`internal/plugin/plugin_test.go`。
- [ ] 运行相关包测试：`go test ./internal/plugin`。
- [ ] 运行相关包 race：`go test -race ./internal/plugin`。
- [ ] 运行相关包 vet：`go vet ./internal/plugin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/plugin/plugin.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/plugin/plugin.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-328 · 圈复杂度 · `newStdioTransport`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-328 |
| Baseline key | `complexity\|internal/plugin/transport_stdio.go\|newStdioTransport` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/plugin/transport_stdio.go` |
| 代码行 | 57 |
| 函数/方法/包 | `newStdioTransport` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`newStdioTransport` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 57-124 行；可见控制流热点：L58 `if strings.TrimSpace(s.Command) == "" {`；L64 `if err != nil {`；L78 `if err != nil {`；L92 `if s.Stderr != nil {`；L101 `if err != nil {`；L108 `if s.LowPriority {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（resolveStdioExecutable）、状态与控制（StartTracked、LowPriorityStarted）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function newStdioTransport（internal/plugin/transport_stdio.go:57-124）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `acquireCodeGraphSlot`（`internal/plugin/codegraph_limit.go:23`，调用点 L63，confidence=0.90，exact-match）；`isCodeGraphSpecName`（`internal/plugin/known_overrides.go:53`，调用点 L62，confidence=0.90，exact-match）；`resolveStdioExecutable`（`internal/plugin/transport_stdio.go:150`，调用点 L77，confidence=0.90，exact-match）；`mergeStdioEnv`（`internal/plugin/transport_stdio.go:365`，调用点 L76，confidence=0.90，exact-match）；`HideWindow`（`internal/proc/hide_other.go:8`，调用点 L82，confidence=0.90，import）；`StartTracked`（`internal/proc/kill_other.go:39`，调用点 L104，confidence=0.90，import）；`LowPriority`（`internal/proc/priority_other.go:11`，调用点 L84，confidence=0.90，import）；`LowPriorityStarted`（`internal/proc/priority_other.go:15`，调用点 L109，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 11 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/plugin`、`internal/proc`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`、`internal/plugin/hotadd_test.go`、`internal/plugin/lazy_test.go`、`internal/plugin/main_test.go`、`internal/plugin/planmode_untrusted_test.go`、`internal/plugin/plugin_test.go`、`internal/plugin/stats_test.go`、`internal/plugin/stdio_cancel_test.go`
- **需同步检查：** `internal/plugin/transport_stdio.go`、`internal/plugin/codegraph_limit.go`、`internal/plugin/known_overrides.go`、`internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`

#### 待修改项

- [ ] 逐段核对 L58、L72、L87、L97、L108 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `newStdioTransport` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/plugin/transport_stdio.go|newStdioTransport` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/plugin/transport_stdio.go:57-124` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `newStdioTransport` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/plugin`。
- [ ] 运行相关包 race：`go test -race ./internal/plugin`。
- [ ] 运行相关包 vet：`go vet ./internal/plugin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/plugin/transport_stdio.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/plugin/transport_stdio.go|newStdioTransport` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-327 · 圈复杂度 · `*httpTransport.readSSEResponse`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-327 |
| Baseline key | `complexity\|internal/plugin/transport_http.go\|*httpTransport.readSSEResponse` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/plugin/transport_http.go` |
| 代码行 | 170 |
| 函数/方法/包 | `*httpTransport.readSSEResponse` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*httpTransport.readSSEResponse` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 170-219 行；可见控制流热点：L178 `if data.Len() == 0 {`；L187 `if resp.ID != id {`；L196 `for sc.Scan() {`；L199 `if res, ok, err := match(); err != nil || ok {`；L205 `if data.Len() > 0 {`；L215 `if res, ok, err := match(); err != nil || ok { // stream ended on a final unterminated event`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（ArchiveSink::Err）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method httpTransport::readSSEResponse（internal/plugin/transport_http.go:170-219）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `httpTransport::call`（`internal/plugin/transport_http.go:52`，调用点 L84，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（2）：** `Registry::Len`（`internal/tool/tool.go:242`，调用点 L178/L205，confidence=0.70，instance-method，2 条边，待源码复核）；`ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L212，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `httpTransport::call`（`internal/plugin/transport_http.go:52`，调用点 L84，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 16 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/plugin`、`internal/tool`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`、`internal/plugin/hotadd_test.go`、`internal/plugin/lazy_test.go`、`internal/plugin/main_test.go`、`internal/plugin/planmode_untrusted_test.go`、`internal/plugin/plugin_test.go`、`internal/plugin/stats_test.go`、`internal/plugin/stdio_cancel_test.go`
- **需同步检查：** `internal/plugin/transport_http.go`、`internal/tool/tool.go`、`internal/automation/archive_sink.go`、`internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`

#### 待修改项

- [ ] 逐段核对 L178、L190、L198、L205、L215 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*httpTransport.readSSEResponse` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/plugin/transport_http.go|*httpTransport.readSSEResponse` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/plugin/transport_http.go:170-219` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*httpTransport.readSSEResponse` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/plugin/cache_test.go`、`internal/plugin/canonicalize_test.go`、`internal/plugin/codegraph_limit_test.go`、`internal/plugin/example_test.go`、`internal/plugin/host_concurrency_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/plugin`。
- [ ] 运行相关包 vet：`go vet ./internal/plugin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/plugin/transport_http.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/plugin/transport_http.go|*httpTransport.readSSEResponse` 下降或消失且无新增/恶化。
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
