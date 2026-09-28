# G40 · internal/tool · bash 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G40 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/tool` |
| 涉及文件 | `internal/tool/builtin/bash.go`<br>`internal/tool/builtin/delete_range.go`<br>`internal/tool/builtin/glob.go`<br>`internal/tool/builtin/multiedit.go`<br>`internal/tool/builtin/preview.go`<br>`internal/tool/builtin/ls.go` |
| 主要治理目标 | 围绕 `internal/tool/builtin/bash.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | tool 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 中：认领前确认同包开发 |
| 冲突依据 | 单文件最多集中 5 条问题 |
| 建议处理顺序 | 总序第 40 组；无同文件组依赖 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 返回值、错误语义和现有调用顺序；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容；持久化、迁移或文件格式兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **40/100**，对象 `hasShellCommandWord` 的分项为严重度 6.0、集中度 5.9、影响面 11.9、可执行性 16.0；最高超限 1.50x，文件内 5 条问题，CodeGraph 记录 1 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 6 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-372` | `complexity\|internal/tool/builtin/bash.go\|hasShellCommandWord` | 圈复杂度 | `internal/tool/builtin/bash.go` | `hasShellCommandWord` | 321 | 15/10 | +5（1.50x） | P3 | 1 可信 + 0 待核 caller / 5 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-382` | `complexity\|internal/tool/builtin/delete_range.go\|deleteRange.preview` | 圈复杂度 | `internal/tool/builtin/delete_range.go` | `deleteRange.preview` | 62 | 19/10 | +9（1.90x） | P3 | 0 可信 + 2 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-385` | `complexity\|internal/tool/builtin/glob.go\|globRecursive` | 圈复杂度 | `internal/tool/builtin/glob.go` | `globRecursive` | 90 | 16/10 | +6（1.60x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 1 待核 callee / 19 unresolved | 待修改 |
| `CX-370` | `complexity\|internal/tool/builtin/bash.go\|bash.Execute` | 圈复杂度 | `internal/tool/builtin/bash.go` | `bash.Execute` | 124 | 14/10 | +4（1.40x） | P3 | 0 可信 + 0 待核 caller / 9 可信 + 6 待核 callee / 0 unresolved | 待修改 |
| `CX-374` | `complexity\|internal/tool/builtin/bash.go\|isShellAssignment` | 圈复杂度 | `internal/tool/builtin/bash.go` | `isShellAssignment` | 438 | 17/10 | +7（1.70x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-371` | `complexity\|internal/tool/builtin/bash.go\|hasUnquotedBackgroundOperator` | 圈复杂度 | `internal/tool/builtin/bash.go` | `hasUnquotedBackgroundOperator` | 278 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-373` | `complexity\|internal/tool/builtin/bash.go\|readShellWord` | 圈复杂度 | `internal/tool/builtin/bash.go` | `readShellWord` | 373 | 14/10 | +4（1.40x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-389` | `complexity\|internal/tool/builtin/multiedit.go\|multiEdit.Execute` | 圈复杂度 | `internal/tool/builtin/multiedit.go` | `multiEdit.Execute` | 64 | 13/10 | +3（1.30x） | P3 | 0 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-390` | `complexity\|internal/tool/builtin/preview.go\|multiEdit.Preview` | 圈复杂度 | `internal/tool/builtin/preview.go` | `multiEdit.Preview` | 91 | 11/10 | +1（1.10x） | P3 | 0 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-387` | `complexity\|internal/tool/builtin/ls.go\|listDir.listRecursive` | 圈复杂度 | `internal/tool/builtin/ls.go` | `listDir.listRecursive` | 80 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 0 可信 + 1 待核 callee / 14 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-372 · 圈复杂度 · `hasShellCommandWord`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-372 |
| Baseline key | `complexity\|internal/tool/builtin/bash.go\|hasShellCommandWord` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 40） |
| 文件 | `internal/tool/builtin/bash.go` |
| 代码行 | 321 |
| 函数/方法/包 | `hasShellCommandWord` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`hasShellCommandWord` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 321-371 行；可见控制流热点：L324 `for i := 0; i < len(s); {`；L330 `switch c {`；L332 `if i+1 < len(s) && (s[i:i+2] == "&&" || s[i:i+2] == "||") {`；L351 `if skipNextWord {`；L358 `if isShellAssignment(word) {`；L365 `if base == "command" || base == "env" {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readShellWord）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function hasShellCommandWord（internal/tool/builtin/bash.go:321-371）
- **可信生产调用方（1）：** `hasExplicitBackgroundKeepalive`（`internal/tool/builtin/bash.go:225`，调用点 L229，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `readShellWord`（`internal/tool/builtin/bash.go:373`，调用点 L346，confidence=0.90，exact-match）；`skipShellRedirect`（`internal/tool/builtin/bash.go:404`，调用点 L341，confidence=0.90，exact-match）；`isShellSpace`（`internal/tool/builtin/bash.go:429`，调用点 L326，confidence=0.90，exact-match）；`isShellAssignment`（`internal/tool/builtin/bash.go:438`，调用点 L358，confidence=0.90，exact-match）；`shellWordBase`（`internal/tool/builtin/bash.go:458`，调用点 L361，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/bash.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L324、L331、L348、L355、L365 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `hasShellCommandWord` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/bash.go|hasShellCommandWord` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/bash.go:321-371` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `hasShellCommandWord` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/bash.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/bash.go|hasShellCommandWord` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-382 · 圈复杂度 · `deleteRange.preview`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-382 |
| Baseline key | `complexity\|internal/tool/builtin/delete_range.go\|deleteRange.preview` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 40） |
| 文件 | `internal/tool/builtin/delete_range.go` |
| 代码行 | 62 |
| 函数/方法/包 | `deleteRange.preview` |
| Baseline / 当前复测 / 阈值 | 19 / 19 / 10 |
| 超出 | +9，1.90x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`deleteRange.preview` 的圈复杂度当前为 **19**，超过限制 **10**（+9，1.90x）。定义范围为 62-145 行；可见控制流热点：L69 `if err := json.Unmarshal(args, &p); err != nil {`；L78 `if p.EndAnchor == "" {`；L93 `if err != nil {`；L109 `if startLine == -1 {`；L119 `if startLine > endLine {`；L140 `if newContent != "" && strings.HasSuffix(original, lineSep) && !strings.HasSuffix(newContent, lineS…`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（readFileEncoded）、持久化/I-O（readFileEncoded）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method deleteRange::preview（internal/tool/builtin/delete_range.go:62-145）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `deleteRange::Execute`（`internal/tool/builtin/delete_range.go:41`，调用点 L42，confidence=0.70，instance-method，待源码复核）；`deleteRange::Preview`（`internal/tool/builtin/delete_range.go:58`，调用点 L59，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `findUniqueLine`（`internal/tool/builtin/delete_range.go:149`，调用点 L105/L112，confidence=0.90，exact-match，2 条边）；`Build`（`internal/diff/diff.go:51`，调用点 L144，confidence=0.90，import）；`readFileEncoded`（`internal/tool/builtin/encoding_helpers.go:13`，调用点 L92，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `deleteRange::Execute`（`internal/tool/builtin/delete_range.go:41`，调用点 L42，confidence=0.70，instance-method，待源码复核）；`deleteRange::Preview`（`internal/tool/builtin/delete_range.go:58`，调用点 L59，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/tool`、`internal/diff`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/delete_range.go`、`internal/diff/diff.go`、`internal/tool/builtin/encoding_helpers.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L69、L83、L106、L116、L140 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `deleteRange.preview` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/delete_range.go|deleteRange.preview` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/delete_range.go:62-145` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `deleteRange.preview` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/delete_range.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/delete_range.go|deleteRange.preview` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-385 · 圈复杂度 · `globRecursive`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-385 |
| Baseline key | `complexity\|internal/tool/builtin/glob.go\|globRecursive` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/tool/builtin/glob.go` |
| 代码行 | 90 |
| 函数/方法/包 | `globRecursive` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`globRecursive` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 90-163 行；可见控制流热点：L96 `if root == "" {`；L118 `if ctx.Err() != nil {`；L124 `if d.IsDir() {`；L137 `if rerr != nil {`；L144 `if len(matches) >= globMaxResults {`；L159 `if truncated {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（ArchiveSink::Err）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function globRecursive（internal/tool/builtin/glob.go:90-163）
- **可信生产调用方（1）：** `globTool::Execute`（`internal/tool/builtin/glob.go:38`，调用点 L60/L74，confidence=0.90，exact-match，2 条边）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `matchGlobSuffix`（`internal/tool/builtin/glob.go:169`，调用点 L140，confidence=0.90，exact-match）；`skipWalkDir`（`internal/tool/builtin/workspace.go:119`，调用点 L125，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L118/L119，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** `globTool::Execute`（`internal/tool/builtin/glob.go:38`，调用点 L60，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 19 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/tool`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/glob.go`、`internal/tool/builtin/workspace.go`、`internal/automation/archive_sink.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L96、L118、L131、L144、L159 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `globRecursive` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/glob.go|globRecursive` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/glob.go:90-163` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `globRecursive` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/glob.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/glob.go|globRecursive` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-370 · 圈复杂度 · `bash.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-370 |
| Baseline key | `complexity\|internal/tool/builtin/bash.go\|bash.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/tool/builtin/bash.go` |
| 代码行 | 124 |
| 函数/方法/包 | `bash.Execute` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`bash.Execute` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 124-211 行；可见控制流热点：L130 `if err := json.Unmarshal(args, &p); err != nil {`；L138 `if !sh.SupportsChaining() && (hasUnquotedSeq(p.Command, "&&") || hasUnquotedSeq(p.Command, "||")) {`；L150 `if !ok {`；L175 `if timeout > 0 {`；L198 `if shouldReapAfterRun(runCtx, sh, p.Command, p.PreserveBackgroundProcesses) {`；L206 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（bash::resolved）、状态与控制（shouldReapAfterRun、setKillTree、Manager::StartForSession）、持久化/I-O（newProgressWriter）、渲染/输出（commandPreview）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method bash::Execute（internal/tool/builtin/bash.go:124-211）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（9）：** `shouldReapAfterRun`（`internal/tool/builtin/bash.go:213`，调用点 L165/L198，confidence=0.90，exact-match，2 条边）；`hasUnquotedSeq`（`internal/tool/builtin/bash.go:257`，调用点 L138，confidence=0.90，exact-match，2 条边）；`FromContext`（`internal/jobs/jobs.go:1485`，调用点 L149，confidence=0.90，import）；`SessionFromContext`（`internal/jobs/jobs.go:1498`，调用点 L156，confidence=0.90，import）；`Command`（`internal/sandbox/seatbelt_darwin.go:16`，调用点 L145，confidence=0.90，import）；`newProgressWriter`（`internal/tool/builtin/bash.go:247`，调用点 L189，confidence=0.90，exact-match）；`commandPreview`（`internal/tool/builtin/bash.go:467`，调用点 L156，confidence=0.90，exact-match）；`bashCommandEnv`（`internal/tool/builtin/bash.go:477`，调用点 L146，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `setKillTree`（`internal/tool/builtin/bash_kill_other.go:14`，调用点 L160/L184，confidence=0.70，exact-match，2 条边，待源码复核）；`reapTree`（`internal/tool/builtin/bash_kill_other.go:28`，调用点 L166/L199，confidence=0.70，exact-match，2 条边，待源码复核）；`Manager::StartForSession`（`internal/jobs/jobs.go:242`，调用点 L156，confidence=0.70，instance-method，待源码复核）；`Shell::SupportsChaining`（`internal/sandbox/shell.go:260`，调用点 L138，confidence=0.70，instance-method，待源码复核）；`bash::resolved`（`internal/tool/builtin/bash.go:105`，调用点 L137，confidence=0.70，instance-method，待源码复核）；`bash::foregroundTimeout`（`internal/tool/builtin/bash.go:236`，调用点 L174，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/tool`、`internal/jobs`、`internal/sandbox`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/tool/builtin/bgjobs_test.go`、`internal/agent/subagent_registry_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/delete_symbol_test.go`、`internal/tool/builtin/workspace_test.go`、`internal/cli/review_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/confine_test.go`
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`
- **需同步检查：** `internal/tool/builtin/bash.go`、`internal/jobs/jobs.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/agent/subagent_registry_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/delete_symbol_test.go`

#### 待修改项

- [ ] 逐段核对 L130、L148、L165、L198、L206 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `bash.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/bash.go|bash.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/bash.go:124-211` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `bash.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/tool/builtin/bgjobs_test.go`、`internal/agent/subagent_registry_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/delete_symbol_test.go`、`internal/tool/builtin/workspace_test.go`。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/bash.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/bash.go|bash.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-374 · 圈复杂度 · `isShellAssignment`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-374 |
| Baseline key | `complexity\|internal/tool/builtin/bash.go\|isShellAssignment` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 37） |
| 文件 | `internal/tool/builtin/bash.go` |
| 代码行 | 438 |
| 函数/方法/包 | `isShellAssignment` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`isShellAssignment` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 438-456 行；可见控制流热点：L440 `if !ok || name == "" {`；L443 `for i := 0; i < len(name); i++ {`；L445 `if i == 0 {`；L446 `if c != '_' && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {`；L451 `if c != '_' && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function isShellAssignment（internal/tool/builtin/bash.go:438-456）
- **可信生产调用方（1）：** `hasShellCommandWord`（`internal/tool/builtin/bash.go:321`，调用点 L358，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/bash.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L440、L443、L445、L446、L451 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `isShellAssignment` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/bash.go|isShellAssignment` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/bash.go:438-456` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `isShellAssignment` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/bash.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/bash.go|isShellAssignment` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-371 · 圈复杂度 · `hasUnquotedBackgroundOperator`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-371 |
| Baseline key | `complexity\|internal/tool/builtin/bash.go\|hasUnquotedBackgroundOperator` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/tool/builtin/bash.go` |
| 代码行 | 278 |
| 函数/方法/包 | `hasUnquotedBackgroundOperator` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`hasUnquotedBackgroundOperator` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 278-319 行；可见控制流热点：L281 `for i := 0; i < len(s); i++ {`；L287 `if c == '\\' {`；L292 `if c == quote {`；L297 `if c == '\'' || c == '"' {`；L304 `if i+1 < len(s) && s[i+1] == '&' {`；L313 `if next == '>' {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function hasUnquotedBackgroundOperator（internal/tool/builtin/bash.go:278-319）
- **可信生产调用方（1）：** `hasExplicitBackgroundKeepalive`（`internal/tool/builtin/bash.go:225`，调用点 L226，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `previousNonSpace`（`internal/tool/builtin/bash.go:411`，调用点 L308，confidence=0.90，exact-match）；`nextNonSpace`（`internal/tool/builtin/bash.go:420`，调用点 L312，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/bash.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L281、L287、L297、L304、L313 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `hasUnquotedBackgroundOperator` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/bash.go|hasUnquotedBackgroundOperator` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/bash.go:278-319` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `hasUnquotedBackgroundOperator` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/bash.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/bash.go|hasUnquotedBackgroundOperator` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-373 · 圈复杂度 · `readShellWord`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-373 |
| Baseline key | `complexity\|internal/tool/builtin/bash.go\|readShellWord` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/tool/builtin/bash.go` |
| 代码行 | 373 |
| 函数/方法/包 | `readShellWord` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`readShellWord` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 373-402 行；可见控制流热点：L375 `for i := start; i < len(s); i++ {`；L380 `switch c {`；L382 `if i+1 < len(s) {`；L386 `case '\'':`；L390 `case '"':`；L392 `if s[i] == '\\' && i+1 < len(s) {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function readShellWord（internal/tool/builtin/bash.go:373-402）
- **可信生产调用方（1）：** `hasShellCommandWord`（`internal/tool/builtin/bash.go:321`，调用点 L346，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `isShellSpace`（`internal/tool/builtin/bash.go:429`，调用点 L377，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/bash.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L375、L380、L386、L390、L392 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `readShellWord` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/bash.go|readShellWord` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/bash.go:373-402` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `readShellWord` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/bash.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/bash.go|readShellWord` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-389 · 圈复杂度 · `multiEdit.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-389 |
| Baseline key | `complexity\|internal/tool/builtin/multiedit.go\|multiEdit.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/tool/builtin/multiedit.go` |
| 代码行 | 64 |
| 函数/方法/包 | `multiEdit.Execute` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`multiEdit.Execute` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 64-122 行；可见控制流热点：L69 `if err := json.Unmarshal(args, &p); err != nil {`；L75 `if len(p.Edits) == 0 {`；L93 `for i, step := range p.Edits {`；L98 `if step.ReplaceAll {`；L108 `case 0:`；L118 `if err := writeFileEncoded(p.Path, content, enc); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（readFileEncoded、writeFileEncoded）、持久化/I-O（readFileEncoded、writeFileEncoded）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method multiEdit::Execute（internal/tool/builtin/multiedit.go:64-122）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `readFileEncoded`（`internal/tool/builtin/encoding_helpers.go:13`，调用点 L83，confidence=0.90，exact-match）；`writeFileEncoded`（`internal/tool/builtin/encoding_helpers.go:23`，调用点 L118，confidence=0.90，exact-match）；`matchLineEndings`（`internal/tool/builtin/encoding_helpers.go:32`，调用点 L97，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/multiedit.go`、`internal/tool/builtin/encoding_helpers.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L69、L79、L94、L107、L118 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `multiEdit.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/multiedit.go|multiEdit.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/multiedit.go:64-122` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `multiEdit.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/multiedit.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/multiedit.go|multiEdit.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-390 · 圈复杂度 · `multiEdit.Preview`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-390 |
| Baseline key | `complexity\|internal/tool/builtin/preview.go\|multiEdit.Preview` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/tool/builtin/preview.go` |
| 代码行 | 91 |
| 函数/方法/包 | `multiEdit.Preview` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`multiEdit.Preview` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 91-134 行；可见控制流热点：L96 `if err := json.Unmarshal(args, &p); err != nil {`；L102 `if len(p.Edits) == 0 {`；L113 `for i, step := range p.Edits {`；L117 `if step.ReplaceAll {`；L124 `switch strings.Count(content, step.OldString) {`；L127 `case 1:`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（readFileEncoded）、持久化/I-O（readFileEncoded）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method multiEdit::Preview（internal/tool/builtin/preview.go:91-134）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `Build`（`internal/diff/diff.go:51`，调用点 L133，confidence=0.90，import）；`readFileEncoded`（`internal/tool/builtin/encoding_helpers.go:13`，调用点 L107，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/tool`、`internal/diff`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/preview.go`、`internal/diff/diff.go`、`internal/tool/builtin/encoding_helpers.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L96、L108、L114、L124、L127 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `multiEdit.Preview` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/preview.go|multiEdit.Preview` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/preview.go:91-134` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `multiEdit.Preview` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/preview.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/preview.go|multiEdit.Preview` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-387 · 圈复杂度 · `listDir.listRecursive`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-387 |
| Baseline key | `complexity\|internal/tool/builtin/ls.go\|listDir.listRecursive` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 32） |
| 文件 | `internal/tool/builtin/ls.go` |
| 代码行 | 80 |
| 函数/方法/包 | `listDir.listRecursive` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`listDir.listRecursive` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 80-122 行；可见控制流热点：L83 `if wErr != nil {`；L89 `if d.IsDir() {`；L91 `case ".git", "node_modules", ".DS_Store", "__pycache__", ".idea", ".vscode":`；L100 `if strings.Count(rel, string(os.PathSeparator)) > 50 {`；L107 `if d.IsDir() {`；L118 `if b.Len() == 0 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method listDir::listRecursive（internal/tool/builtin/ls.go:80-122）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `listDir::Execute`（`internal/tool/builtin/ls.go:35`，调用点 L52，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `Registry::Len`（`internal/tool/tool.go:242`，调用点 L118，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `listDir::Execute`（`internal/tool/builtin/ls.go:35`，调用点 L52，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 14 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/ls.go`、`internal/tool/tool.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L83、L90、L96、L107、L118 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `listDir.listRecursive` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/ls.go|listDir.listRecursive` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/ls.go:80-122` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `listDir.listRecursive` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/ls.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/ls.go|listDir.listRecursive` 下降或消失且无新增/恶化。
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
