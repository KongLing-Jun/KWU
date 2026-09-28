# G08 · internal/tool · webfetch 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G08 |
| 问题数量 | 10 |
| 整体优先级 | P1 |
| 涉及模块 | `internal/tool` |
| 涉及文件 | `internal/tool/builtin/webfetch.go`<br>`internal/tool/builtin/grep.go`<br>`internal/tool/builtin/delete_symbol.go`<br>`internal/tool/builtin/completestep.go` |
| 主要治理目标 | 围绕 `internal/tool/builtin/webfetch.go` 等治理对象降低复杂度/体量，统一处理 3 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | tool 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 低：仍需开始前复查开放 PR |
| 冲突依据 | 未发现静态热点或本地远端引用重叠 |
| 建议处理顺序 | 总序第 8 组；先于 G32 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 并发、取消或事件顺序；返回值、错误语义和现有调用顺序；持久化、迁移或文件格式兼容；会话、状态或恢复语义 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **60/100**，对象 `ssrfGuardedTransport` 的分项为严重度 16.3、集中度 15.3、影响面 8.5、可执行性 20.0；最高超限 2.40x，文件内 4 条问题，CodeGraph 记录 1 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数、嵌套深度，必须合并治理。

### 治理边界与组间关系

本组涉及 4 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G32，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-391` | `complexity\|internal/tool/builtin/webfetch.go\|ssrfGuardedTransport` | 圈复杂度 | `internal/tool/builtin/webfetch.go` | `ssrfGuardedTransport` | 58 | 24/10 | +14（2.40x） | P1 | 1 可信 + 0 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `FN-051` | `func_lines\|internal/tool/builtin/webfetch.go\|ssrfGuardedTransport` | 函数有效行数 | `internal/tool/builtin/webfetch.go` | `ssrfGuardedTransport` | 58 | 95/80 | +15（1.19x） | P1 | 1 可信 + 0 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `ND-020` | `nesting_depth\|internal/tool/builtin/webfetch.go\|ssrfGuardedTransport` | 嵌套深度 | `internal/tool/builtin/webfetch.go` | `ssrfGuardedTransport` | 58 | 6/4 | +2（1.50x） | P1 | 1 可信 + 0 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-386` | `complexity\|internal/tool/builtin/grep.go\|grepTool.Execute` | 圈复杂度 | `internal/tool/builtin/grep.go` | `grepTool.Execute` | 87 | 27/10 | +17（2.70x） | P2 | 0 可信 + 0 待核 caller / 8 可信 + 4 待核 callee / 41 unresolved | 待修改 |
| `FN-050` | `func_lines\|internal/tool/builtin/grep.go\|grepTool.Execute` | 函数有效行数 | `internal/tool/builtin/grep.go` | `grepTool.Execute` | 87 | 118/80 | +38（1.48x） | P2 | 0 可信 + 0 待核 caller / 8 可信 + 4 待核 callee / 41 unresolved | 待修改 |
| `CX-384` | `complexity\|internal/tool/builtin/delete_symbol.go\|collectSymbols` | 圈复杂度 | `internal/tool/builtin/delete_symbol.go` | `collectSymbols` | 209 | 19/10 | +9（1.90x） | P2 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `ND-019` | `nesting_depth\|internal/tool/builtin/delete_symbol.go\|collectSymbols` | 嵌套深度 | `internal/tool/builtin/delete_symbol.go` | `collectSymbols` | 209 | 5/4 | +1（1.25x） | P2 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-379` | `complexity\|internal/tool/builtin/completestep.go\|verifyStepEvidence` | 圈复杂度 | `internal/tool/builtin/completestep.go` | `verifyStepEvidence` | 144 | 18/10 | +8（1.80x） | P2 | 1 可信 + 0 待核 caller / 5 可信 + 6 待核 callee / 0 unresolved | 待修改 |
| `CX-383` | `complexity\|internal/tool/builtin/delete_symbol.go\|deleteSymbol.findSymbol` | 圈复杂度 | `internal/tool/builtin/delete_symbol.go` | `deleteSymbol.findSymbol` | 144 | 17/10 | +7（1.70x） | P3 | 0 可信 + 2 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-392` | `complexity\|internal/tool/builtin/webfetch.go\|webFetch.Execute` | 圈复杂度 | `internal/tool/builtin/webfetch.go` | `webFetch.Execute` | 238 | 12/10 | +2（1.20x） | P3 | 0 可信 + 0 待核 caller / 4 可信 + 1 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-391 · 圈复杂度 · `ssrfGuardedTransport`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-391 |
| Baseline key | `complexity\|internal/tool/builtin/webfetch.go\|ssrfGuardedTransport` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P1，治理对象得分 60） |
| 文件 | `internal/tool/builtin/webfetch.go` |
| 代码行 | 58 |
| 函数/方法/包 | `ssrfGuardedTransport` |
| Baseline / 当前复测 / 阈值 | 24 / 24 / 10 |
| 超出 | +14，2.40x |
| 同一治理对象关联问题 | `FN-051`、`ND-020` |

#### 明确的问题描述

`ssrfGuardedTransport` 的圈复杂度当前为 **24**，超过限制 **10**（+14，2.40x）。定义范围为 58-180 行；可见控制流热点：L66 `if err != nil {`；L85 `if proxyURL != "" {`；L98 `if err != nil {`；L131 `if err := connectReq.Write(proxyConn); err != nil {`；L156 `if pu.User != nil {`；L168 `if ip := net.ParseIP(host); ip != nil && blockedFetchIP(ip) {`。CodeGraph 记录的下游职责候选主要涉及 网络/协议（blockedFetchIP）、任务/并发（blockedFetchIP）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ssrfGuardedTransport（internal/tool/builtin/webfetch.go:58-180）
- **可信生产调用方（1）：** `webFetchRoundTripper::RoundTrip`（`internal/tool/builtin/webfetch.go:186`，调用点 L191，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `blockedFetchIP`（`internal/tool/builtin/webfetch.go:215`，调用点 L74/L104/L168，confidence=0.70，exact-match，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** `webFetchRoundTripper::RoundTrip`（`internal/tool/builtin/webfetch.go:186`，调用点 L191，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/webfetch.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L66、L87、L112、L149、L168 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 网络/协议、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `ssrfGuardedTransport` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-391、FN-051、ND-020 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/webfetch.go|ssrfGuardedTransport` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/webfetch.go:58-180` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ssrfGuardedTransport` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-391`、`FN-051`、`ND-020` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 race：`go test -race ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/webfetch.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/webfetch.go|ssrfGuardedTransport` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-051 · 函数有效行数 · `ssrfGuardedTransport`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-051 |
| Baseline key | `func_lines\|internal/tool/builtin/webfetch.go\|ssrfGuardedTransport` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P3，治理对象得分 60） |
| 文件 | `internal/tool/builtin/webfetch.go` |
| 代码行 | 58 |
| 函数/方法/包 | `ssrfGuardedTransport` |
| Baseline / 当前复测 / 阈值 | 95 / 95 / 80 |
| 超出 | +15，1.19x |
| 同一治理对象关联问题 | `CX-391`、`ND-020` |

#### 明确的问题描述

`ssrfGuardedTransport` 的有效代码行 **95**，超过限制 **80**（+15）。CodeGraph 定义物理范围为 58-180 行，并记录 1 个直接下游候选；候选职责分布为 网络/协议（blockedFetchIP）、任务/并发（blockedFetchIP）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ssrfGuardedTransport（internal/tool/builtin/webfetch.go:58-180）
- **可信生产调用方（1）：** `webFetchRoundTripper::RoundTrip`（`internal/tool/builtin/webfetch.go:186`，调用点 L191，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `blockedFetchIP`（`internal/tool/builtin/webfetch.go:215`，调用点 L74/L104/L168，confidence=0.70，exact-match，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** `webFetchRoundTripper::RoundTrip`（`internal/tool/builtin/webfetch.go:186`，调用点 L191，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/webfetch.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 把 `ssrfGuardedTransport` 的 95 个有效行按 网络/协议、任务/并发 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L61 `directDialContext handles SSRF-protected direct connection (no proxy).`、L94 `go through the trusted proxy which resolves them.`、L115 `CONNECT the ORIGINAL hostname through the proxy, letting`、L154 `proxy — the same boundary as the HTTP CONNECT path above.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-391、FN-051、ND-020 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/tool/builtin/webfetch.go|ssrfGuardedTransport` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/webfetch.go:58-180` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ssrfGuardedTransport` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-391`、`FN-051`、`ND-020` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 race：`go test -race ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/webfetch.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/tool/builtin/webfetch.go|ssrfGuardedTransport` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-020 · 嵌套深度 · `ssrfGuardedTransport`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-020 |
| Baseline key | `nesting_depth\|internal/tool/builtin/webfetch.go\|ssrfGuardedTransport` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P2，治理对象得分 60） |
| 文件 | `internal/tool/builtin/webfetch.go` |
| 代码行 | 58 |
| 函数/方法/包 | `ssrfGuardedTransport` |
| Baseline / 当前复测 / 阈值 | 6 / 6 / 4 |
| 超出 | +2，1.50x |
| 同一治理对象关联问题 | `CX-391`、`FN-051` |

#### 明确的问题描述

`ssrfGuardedTransport` 的最大嵌套深度为 **6**，超过限制 **4**。嵌套相关控制点包括：L66 `if err != nil {`；L85 `if proxyURL != "" {`；L98 `if err != nil {`；L131 `if err := connectReq.Write(proxyConn); err != nil {`；L156 `if pu.User != nil {`；L168 `if ip := net.ParseIP(host); ip != nil && blockedFetchIP(ip) {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ssrfGuardedTransport（internal/tool/builtin/webfetch.go:58-180）
- **可信生产调用方（1）：** `webFetchRoundTripper::RoundTrip`（`internal/tool/builtin/webfetch.go:186`，调用点 L191，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `blockedFetchIP`（`internal/tool/builtin/webfetch.go:215`，调用点 L74/L104/L168，confidence=0.70，exact-match，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** `webFetchRoundTripper::RoundTrip`（`internal/tool/builtin/webfetch.go:186`，调用点 L191，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/webfetch.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 从 L66、L87、L112、L149、L168 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-391、FN-051、ND-020 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/tool/builtin/webfetch.go|ssrfGuardedTransport` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/webfetch.go:58-180` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ssrfGuardedTransport` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-391`、`FN-051`、`ND-020` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 race：`go test -race ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/webfetch.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/tool/builtin/webfetch.go|ssrfGuardedTransport` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-386 · 圈复杂度 · `grepTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-386 |
| Baseline key | `complexity\|internal/tool/builtin/grep.go\|grepTool.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 51） |
| 文件 | `internal/tool/builtin/grep.go` |
| 代码行 | 87 |
| 函数/方法/包 | `grepTool.Execute` |
| Baseline / 当前复测 / 阈值 | 27 / 27 / 10 |
| 超出 | +17，2.70x |
| 同一治理对象关联问题 | `FN-050` |

#### 明确的问题描述

`grepTool.Execute` 的圈复杂度当前为 **27**，超过限制 **10**（+17，2.70x）。定义范围为 87-234 行；可见控制流热点：L93 `if err := json.Unmarshal(args, &p); err != nil {`；L112 `if err != nil {`；L155 `if err != nil {`；L189 `if re.MatchString(line) {`；L211 `if err != nil {`；L224 `if searchFile(path) == io.EOF {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Decode、Decoder）、状态与控制（grepTool::runRipgrep）、持久化/I-O（readablePath、ArchiveSink::Err）、渲染/输出（formatGrep）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method grepTool::Execute（internal/tool/builtin/grep.go:87-234）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `Detect`（`internal/fileutil/encoding/encoding.go:45`，调用点 L148，confidence=0.90，import）；`DetectQuick`（`internal/fileutil/encoding/encoding.go:77`，调用点 L137，confidence=0.90，import）；`Decode`（`internal/fileutil/encoding/encoding.go:122`，调用点 L159，confidence=0.90，import）；`Decoder`（`internal/fileutil/encoding/encoding.go:149`，调用点 L164，confidence=0.90，import）；`readablePath`（`internal/tool/builtin/confine.go:106`，调用点 L100，confidence=0.90，exact-match）；`newWalkIgnorer`（`internal/tool/builtin/gitignore.go:40`，调用点 L206，confidence=0.90，exact-match）；`grepTimeout`（`internal/tool/builtin/grep.go:32`，调用点 L104，confidence=0.90，exact-match）；`formatGrep`（`internal/tool/builtin/grep.go:43`，调用点 L233，confidence=0.90，exact-match）
- **待源码复核被调候选（4）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L208/L209，confidence=0.70，instance-method，2 条边，待源码复核）；`walkIgnorer::skip`（`internal/tool/builtin/gitignore.go:84`，调用点 L215/L221，confidence=0.70，instance-method，2 条边，待源码复核）；`walkIgnorer::enter`（`internal/tool/builtin/gitignore.go:69`，调用点 L218，confidence=0.70，instance-method，待源码复核）；`grepTool::runRipgrep`（`internal/tool/builtin/grep.go:239`，调用点 L109，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 41 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/tool`、`internal/fileutil`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/grep.go`、`internal/fileutil/encoding/encoding.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L93、L138、L183、L208、L224 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `grepTool.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-386、FN-050 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/grep.go|grepTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/grep.go:87-234` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `grepTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-386`、`FN-050` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 race：`go test -race ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/grep.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/grep.go|grepTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-050 · 函数有效行数 · `grepTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-050 |
| Baseline key | `func_lines\|internal/tool/builtin/grep.go\|grepTool.Execute` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 51） |
| 文件 | `internal/tool/builtin/grep.go` |
| 代码行 | 87 |
| 函数/方法/包 | `grepTool.Execute` |
| Baseline / 当前复测 / 阈值 | 118 / 118 / 80 |
| 超出 | +38，1.48x |
| 同一治理对象关联问题 | `CX-386` |

#### 明确的问题描述

`grepTool.Execute` 的有效代码行 **118**，超过限制 **80**（+38）。CodeGraph 定义物理范围为 87-234 行，并记录 12 个直接下游候选；候选职责分布为 配置/解析（Decode、Decoder）、状态与控制（grepTool::runRipgrep）、持久化/I-O（readablePath、ArchiveSink::Err）、渲染/输出（formatGrep）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method grepTool::Execute（internal/tool/builtin/grep.go:87-234）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `Detect`（`internal/fileutil/encoding/encoding.go:45`，调用点 L148，confidence=0.90，import）；`DetectQuick`（`internal/fileutil/encoding/encoding.go:77`，调用点 L137，confidence=0.90，import）；`Decode`（`internal/fileutil/encoding/encoding.go:122`，调用点 L159，confidence=0.90，import）；`Decoder`（`internal/fileutil/encoding/encoding.go:149`，调用点 L164，confidence=0.90，import）；`readablePath`（`internal/tool/builtin/confine.go:106`，调用点 L100，confidence=0.90，exact-match）；`newWalkIgnorer`（`internal/tool/builtin/gitignore.go:40`，调用点 L206，confidence=0.90，exact-match）；`grepTimeout`（`internal/tool/builtin/grep.go:32`，调用点 L104，confidence=0.90，exact-match）；`formatGrep`（`internal/tool/builtin/grep.go:43`，调用点 L233，confidence=0.90，exact-match）
- **待源码复核被调候选（4）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L208/L209，confidence=0.70，instance-method，2 条边，待源码复核）；`walkIgnorer::skip`（`internal/tool/builtin/gitignore.go:84`，调用点 L215/L221，confidence=0.70，instance-method，2 条边，待源码复核）；`walkIgnorer::enter`（`internal/tool/builtin/gitignore.go:69`，调用点 L218，confidence=0.70，instance-method，待源码复核）；`grepTool::runRipgrep`（`internal/tool/builtin/grep.go:239`，调用点 L109，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 41 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/tool`、`internal/fileutil`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/grep.go`、`internal/fileutil/encoding/encoding.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 把 `grepTool.Execute` 的 118 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L119 `Reused across the serial walk so each file doesn't re-allocate ~72 KiB.`、L144 `Detect encoding from the peek alone — sufficient for the`、L152 `UTF-16 needs full-file decode (multi-byte units span the`、L175 `UTF-8 or LossyUTF8 — no transformation needed.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-386、FN-050 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/tool/builtin/grep.go|grepTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/grep.go:87-234` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `grepTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-386`、`FN-050` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 race：`go test -race ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/grep.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/tool/builtin/grep.go|grepTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-384 · 圈复杂度 · `collectSymbols`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-384 |
| Baseline key | `complexity\|internal/tool/builtin/delete_symbol.go\|collectSymbols` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 49） |
| 文件 | `internal/tool/builtin/delete_symbol.go` |
| 代码行 | 209 |
| 函数/方法/包 | `collectSymbols` |
| Baseline / 当前复测 / 阈值 | 19 / 19 / 10 |
| 超出 | +9，1.90x |
| 同一治理对象关联问题 | `ND-019` |

#### 明确的问题描述

`collectSymbols` 的圈复杂度当前为 **19**，超过限制 **10**（+9，1.90x）。定义范围为 209-284 行；可见控制流热点：L211 `for _, decl := range f.Decls {`；L221 `if d.Doc != nil {`；L236 `case *ast.GenDecl:`；L239 `case *ast.TypeSpec:`；L257 `if d.Tok == token.CONST {`；L268 `for _, ident := range s.Names {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function collectSymbols（internal/tool/builtin/delete_symbol.go:209-284）
- **可信生产调用方（1）：** `deleteSymbol::findSymbol`（`internal/tool/builtin/delete_symbol.go:144`，调用点 L151，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `specDoc`（`internal/tool/builtin/delete_symbol.go:291`，调用点 L251/L265，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `deleteSymbol::Execute`（`internal/tool/builtin/delete_symbol.go:57`，调用点 L83，confidence=0.70，instance-method，待源码复核）；`deleteSymbol::Preview`（`internal/tool/builtin/delete_symbol.go:103`，调用点 L129，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/delete_symbol.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L211、L224、L238、L255、L268 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `collectSymbols` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-384、ND-019 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/delete_symbol.go|collectSymbols` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/delete_symbol.go:209-284` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `collectSymbols` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-384`、`ND-019` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/delete_symbol.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/delete_symbol.go|collectSymbols` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-019 · 嵌套深度 · `collectSymbols`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-019 |
| Baseline key | `nesting_depth\|internal/tool/builtin/delete_symbol.go\|collectSymbols` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 49） |
| 文件 | `internal/tool/builtin/delete_symbol.go` |
| 代码行 | 209 |
| 函数/方法/包 | `collectSymbols` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-384` |

#### 明确的问题描述

`collectSymbols` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L211 `for _, decl := range f.Decls {`；L221 `if d.Doc != nil {`；L236 `case *ast.GenDecl:`；L239 `case *ast.TypeSpec:`；L257 `if d.Tok == token.CONST {`；L268 `for _, ident := range s.Names {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function collectSymbols（internal/tool/builtin/delete_symbol.go:209-284）
- **可信生产调用方（1）：** `deleteSymbol::findSymbol`（`internal/tool/builtin/delete_symbol.go:144`，调用点 L151，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `specDoc`（`internal/tool/builtin/delete_symbol.go:291`，调用点 L251/L265，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `deleteSymbol::Execute`（`internal/tool/builtin/delete_symbol.go:57`，调用点 L83，confidence=0.70，instance-method，待源码复核）；`deleteSymbol::Preview`（`internal/tool/builtin/delete_symbol.go:103`，调用点 L129，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/delete_symbol.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 从 L211、L224、L238、L255、L268 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-384、ND-019 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/tool/builtin/delete_symbol.go|collectSymbols` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/delete_symbol.go:209-284` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `collectSymbols` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-384`、`ND-019` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/delete_symbol.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/tool/builtin/delete_symbol.go|collectSymbols` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-379 · 圈复杂度 · `verifyStepEvidence`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-379 |
| Baseline key | `complexity\|internal/tool/builtin/completestep.go\|verifyStepEvidence` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 45） |
| 文件 | `internal/tool/builtin/completestep.go` |
| 代码行 | 144 |
| 函数/方法/包 | `verifyStepEvidence` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`verifyStepEvidence` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 144-185 行；可见控制流热点：L146 `if !ok {`；L151 `case "verification":`；L156 `if !ledger.HasSuccessfulCommand(command) && !verifyCommandFromSession(ctx, command) {`；L165 `if len(e.Paths) == 0 {`；L172 `case "files":`；L180 `case "manual":`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（Ledger::HasSuccessfulWrite、Ledger::HasSuccessfulReadOrWrite）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function verifyStepEvidence（internal/tool/builtin/completestep.go:144-185）
- **可信生产调用方（1）：** `completeStep::Execute`（`internal/tool/builtin/completestep.go:86`，调用点 L116，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `verifyPathsFromSession`（`internal/tool/builtin/completestep.go:316`，调用点 L168/L176，confidence=0.90，exact-match，2 条边）；`receiptHint`（`internal/tool/builtin/completestep.go:337`，调用点 L169/L177，confidence=0.90，exact-match，2 条边）；`FromContext`（`internal/evidence/evidence.go:544`，调用点 L145，confidence=0.90，import）；`verifyCommandFromSession`（`internal/tool/builtin/completestep.go:281`，调用点 L156，confidence=0.90，exact-match）；`allCommandHints`（`internal/tool/builtin/completestep.go:351`，调用点 L160，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `Ledger::TouchedPaths`（`internal/evidence/evidence.go:146`，调用点 L169/L177，confidence=0.80，instance-method，2 条边，待源码复核）；`Ledger::HasSuccessfulCommand`（`internal/evidence/evidence.go:94`，调用点 L156，confidence=0.80，instance-method，待源码复核）；`Ledger::HasFailedCommand`（`internal/evidence/evidence.go:111`，调用点 L157，confidence=0.80，instance-method，待源码复核）；`Ledger::HasSuccessfulBashMentioningPaths`（`internal/evidence/evidence.go:173`，调用点 L176，confidence=0.80，instance-method，待源码复核）；`Ledger::HasSuccessfulWrite`（`internal/evidence/evidence.go:331`，调用点 L168，confidence=0.80，instance-method，待源码复核）；`Ledger::HasSuccessfulReadOrWrite`（`internal/evidence/evidence.go:335`，调用点 L176，confidence=0.80，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `completeStep::Execute`（`internal/tool/builtin/completestep.go:86`，调用点 L116，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/tool`、`internal/evidence`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/completestep.go`、`internal/evidence/evidence.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L146、L151、L164、L172、L180 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `verifyStepEvidence` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/completestep.go|verifyStepEvidence` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/completestep.go:144-185` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `verifyStepEvidence` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/completestep.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/completestep.go|verifyStepEvidence` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-383 · 圈复杂度 · `deleteSymbol.findSymbol`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-383 |
| Baseline key | `complexity\|internal/tool/builtin/delete_symbol.go\|deleteSymbol.findSymbol` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 40） |
| 文件 | `internal/tool/builtin/delete_symbol.go` |
| 代码行 | 144 |
| 函数/方法/包 | `deleteSymbol.findSymbol` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`deleteSymbol.findSymbol` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 144-207 行；可见控制流热点：L147 `if err != nil {`；L159 `if len(byName) == 0 {`；L167 `if m.kind == kind {`；L178 `for _, m := range filtered {`；L189 `if len(filtered) > 1 {`；L202 `if len(filtered[0].siblings) > 1 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method deleteSymbol::findSymbol（internal/tool/builtin/delete_symbol.go:144-207）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `deleteSymbol::Execute`（`internal/tool/builtin/delete_symbol.go:57`，调用点 L83，confidence=0.70，instance-method，待源码复核）；`deleteSymbol::Preview`（`internal/tool/builtin/delete_symbol.go:103`，调用点 L129，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `collectSymbols`（`internal/tool/builtin/delete_symbol.go:209`，调用点 L151，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `deleteSymbol::Execute`（`internal/tool/builtin/delete_symbol.go:57`，调用点 L83，confidence=0.70，instance-method，待源码复核）；`deleteSymbol::Preview`（`internal/tool/builtin/delete_symbol.go:103`，调用点 L129，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/delete_symbol.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L147、L164、L176、L183、L202 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `deleteSymbol.findSymbol` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/delete_symbol.go|deleteSymbol.findSymbol` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/delete_symbol.go:144-207` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `deleteSymbol.findSymbol` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/delete_symbol.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/delete_symbol.go|deleteSymbol.findSymbol` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-392 · 圈复杂度 · `webFetch.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-392 |
| Baseline key | `complexity\|internal/tool/builtin/webfetch.go\|webFetch.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/tool/builtin/webfetch.go` |
| 代码行 | 238 |
| 函数/方法/包 | `webFetch.Execute` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`webFetch.Execute` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 238-286 行；可见控制流热点：L242 `if err := json.Unmarshal(args, &p); err != nil {`；L245 `if p.URL == "" {`；L256 `if err != nil {`；L265 `if err != nil {`；L277 `if strings.Contains(ct, "text/html") || looksLikeHTML(out) {`；L281 `if out == "" {`。CodeGraph 记录的下游职责候选主要涉及 权限/安全（ssrfGuardedClient）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method webFetch::Execute（internal/tool/builtin/webfetch.go:238-286）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `ssrfGuardedClient`（`internal/tool/builtin/webfetch.go:194`，调用点 L264，confidence=0.90，exact-match）；`looksLikeHTML`（`internal/tool/builtin/webfetch.go:290`，调用点 L277，confidence=0.90，exact-match）；`htmlToText`（`internal/tool/builtin/webfetch.go:311`，调用点 L278，confidence=0.90，exact-match）；`contentTypeShort`（`internal/tool/builtin/webfetch.go:334`，调用点 L284，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Registry::Get`（`internal/tool/tool.go:233`，调用点 L275，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/webfetch.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L242、L249、L265、L271、L281 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 权限/安全 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `webFetch.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/webfetch.go|webFetch.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/webfetch.go:238-286` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `webFetch.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G32 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/webfetch.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/webfetch.go|webFetch.Execute` 下降或消失且无新增/恶化。
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
