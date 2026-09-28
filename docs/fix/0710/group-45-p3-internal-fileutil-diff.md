# G45 · internal/fileutil · diff 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G45 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/fileutil`、`cmd/suiyuan-plugin-example`、`internal/diff`、`internal/expert` |
| 涉及文件 | `internal/fileutil/encoding/encoding.go`<br>`cmd/suiyuan-plugin-example/main.go`<br>`internal/diff/diff.go`<br>`internal/expert/store.go`<br>`internal/expert/definition_catalog.go` |
| 主要治理目标 | 围绕 `internal/diff/diff.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | fileutil / cmd/suiyuan-plugin-example / diff / expert 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 低：仍需开始前复查开放 PR |
| 冲突依据 | 未发现静态热点或本地远端引用重叠 |
| 建议处理顺序 | 总序第 45 组；无同文件组依赖 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 持久化、迁移或文件格式兼容；会话、状态或恢复语义；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **39/100**，对象 `Detect` 的分项为严重度 3.6、集中度 3.6、影响面 11.5、可执行性 20.0；最高超限 1.40x，文件内 2 条问题，CodeGraph 记录 7 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 5 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-262` | `complexity\|internal/fileutil/encoding/encoding.go\|Detect` | 圈复杂度 | `internal/fileutil/encoding/encoding.go` | `Detect` | 45 | 14/10 | +4（1.40x） | P3 | 7 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-008` | `complexity\|cmd/suiyuan-plugin-example/main.go\|handleLine` | 圈复杂度 | `cmd/suiyuan-plugin-example/main.go` | `handleLine` | 102 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 4 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-242` | `complexity\|internal/diff/diff.go\|myers` | 圈复杂度 | `internal/diff/diff.go` | `myers` | 175 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-259` | `complexity\|internal/expert/store.go\|listDefinitions` | 圈复杂度 | `internal/expert/store.go` | `listDefinitions` | 518 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 4 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-244` | `complexity\|internal/diff/diff.go\|unified` | 圈复杂度 | `internal/diff/diff.go` | `unified` | 266 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-258` | `complexity\|internal/expert/store.go\|migrateFirstPrompt` | 圈复杂度 | `internal/expert/store.go` | `migrateFirstPrompt` | 247 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-260` | `complexity\|internal/expert/store.go\|filterDefinitions` | 圈复杂度 | `internal/expert/store.go` | `filterDefinitions` | 655 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-263` | `complexity\|internal/fileutil/encoding/encoding.go\|DetectQuick` | 圈复杂度 | `internal/fileutil/encoding/encoding.go` | `DetectQuick` | 77 | 11/10 | +1（1.10x） | P3 | 3 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-243` | `complexity\|internal/diff/diff.go\|backtrack` | 圈复杂度 | `internal/diff/diff.go` | `backtrack` | 215 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-257` | `complexity\|internal/expert/definition_catalog.go\|definitionSystemPrompt` | 圈复杂度 | `internal/expert/definition_catalog.go` | `definitionSystemPrompt` | 75 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-262 · 圈复杂度 · `Detect`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-262 |
| Baseline key | `complexity\|internal/fileutil/encoding/encoding.go\|Detect` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/fileutil/encoding/encoding.go` |
| 代码行 | 45 |
| 函数/方法/包 | `Detect` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Detect` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 45-70 行；可见控制流热点：L46 `switch {`；L47 `case len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF:`；L49 `case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE:`；L57 `if k, ok := DetectUTF16NoBOM(data); ok {`；L60 `if utf8.Valid(data) {`；L66 `if _, _, err := transform.Bytes(dec, data); err == nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function Detect（internal/fileutil/encoding/encoding.go:45-70）
- **可信生产调用方（7）：** `App::ReadFile`（`desktop/app.go:6599`，调用点 L6651/L6674，confidence=0.90，import，2 条边）；`Store::detectEncoding`（`internal/checkpoint/checkpoint.go:156`，调用点 L165，confidence=0.90，import）；`detectCurrentEncoding`（`internal/checkpoint/checkpoint.go:293`，调用点 L298，confidence=0.90，import）；`readFileEncoded`（`internal/tool/builtin/encoding_helpers.go:13`，调用点 L18，confidence=0.90，import）；`grepTool::Execute`（`internal/tool/builtin/grep.go:87`，调用点 L148，confidence=0.90，import）；`writeFile::Preview`（`internal/tool/builtin/preview.go:26`，调用点 L41，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `DetectUTF16NoBOM`（`internal/fileutil/encoding/encoding.go:94`，调用点 L57，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::ReadFile`（`desktop/app.go:6599`，调用点 L6651，confidence=0.90，import，待源码复核）；`deleteRange::Execute`（`internal/tool/builtin/delete_range.go:41`，调用点 L48，confidence=0.90，exact-match，待源码复核）；`editFile::Execute`（`internal/tool/builtin/editfile.go:34`，调用点 L54，confidence=0.90，exact-match，待源码复核）；`grepTool::Execute`（`internal/tool/builtin/grep.go:87`，调用点 L148，confidence=0.90，import，待源码复核）；`multiEdit::Execute`（`internal/tool/builtin/multiedit.go:64`，调用点 L83，confidence=0.90，exact-match，待源码复核）；`writeFile::Preview`（`internal/tool/builtin/preview.go:26`，调用点 L41，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 7 个文件；`internal/fileutil`、`desktop`、`internal/checkpoint`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/fileutil/encoding/encoding_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/fileutil/encoding/encoding.go`、`desktop/app.go`、`internal/checkpoint/checkpoint.go`、`internal/tool/builtin/encoding_helpers.go`、`internal/fileutil/encoding/encoding_test.go`

#### 待修改项

- [ ] 逐段核对 L46、L49、L51、L60、L66 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `Detect` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/fileutil/encoding/encoding.go|Detect` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/fileutil/encoding/encoding.go:45-70` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Detect` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/fileutil/encoding/encoding_test.go`。
- [ ] 运行相关包测试：`go test ./internal/fileutil/encoding`。
- [ ] 运行相关包 vet：`go vet ./internal/fileutil/encoding`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/fileutil/encoding/encoding.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/fileutil/encoding/encoding.go|Detect` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-008 · 圈复杂度 · `handleLine`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-008 |
| Baseline key | `complexity\|cmd/suiyuan-plugin-example/main.go\|handleLine` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `cmd/suiyuan-plugin-example/main.go` |
| 代码行 | 102 |
| 函数/方法/包 | `handleLine` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`handleLine` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 102-152 行；可见控制流热点：L104 `if len(line) == 0 {`；L112 `if req.ID == nil {`；L128 `case "tools/list":`；L132 `case "prompts/list":`；L138 `case "resources/read":`；L148 `if _, err := w.Write(append(b, '\n')); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readResource）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function handleLine（cmd/suiyuan-plugin-example/main.go:102-152）
- **可信生产调用方（1）：** `serve`（`cmd/suiyuan-plugin-example/main.go:81`，调用点 L89，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `toolList`（`cmd/suiyuan-plugin-example/main.go:212`，调用点 L129，confidence=0.90，exact-match）；`callTool`（`cmd/suiyuan-plugin-example/main.go:232`，调用点 L131，confidence=0.90，exact-match）；`promptList`（`cmd/suiyuan-plugin-example/main.go:264`，调用点 L133，confidence=0.90，exact-match）；`resourceList`（`cmd/suiyuan-plugin-example/main.go:307`，调用点 L137，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `getPrompt`（`cmd/suiyuan-plugin-example/main.go:276`，调用点 L135，confidence=0.70，exact-match，待源码复核）；`readResource`（`cmd/suiyuan-plugin-example/main.go:316`，调用点 L139，confidence=0.70，exact-match，待源码复核）；`trimSpace`（`cmd/suiyuan-plugin-example/main.go:347`，调用点 L103，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `main`（`cmd/suiyuan-plugin-example/main.go:44`，调用点 L47，confidence=0.90，exact-match，待源码复核）；`serve`（`cmd/suiyuan-plugin-example/main.go:81`，调用点 L89，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`cmd/suiyuan-plugin-example`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `cmd/suiyuan-plugin-example/main.go`

#### 待修改项

- [ ] 逐段核对 L104、L117、L130、L136、L148 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `handleLine` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|cmd/suiyuan-plugin-example/main.go|handleLine` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`cmd/suiyuan-plugin-example/main.go:102-152` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `handleLine` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 补充或确认覆盖当前职责、错误分支和边界条件的聚焦测试；不得仅依赖全量测试。
- [ ] 运行相关包测试：`go test ./cmd/suiyuan-plugin-example`。
- [ ] 运行相关包 vet：`go vet ./cmd/suiyuan-plugin-example`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- cmd/suiyuan-plugin-example/main.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|cmd/suiyuan-plugin-example/main.go|handleLine` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-242 · 圈复杂度 · `myers`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-242 |
| Baseline key | `complexity\|internal/diff/diff.go\|myers` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/diff/diff.go` |
| 代码行 | 175 |
| 函数/方法/包 | `myers` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`myers` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 175-210 行；可见控制流热点：L177 `if n == 0 && m == 0 {`；L185 `for d := 0; d <= maxD; d++ {`；L189 `for k := -d; k <= d; k += 2 {`；L193 `if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {`；L199 `for x < n && y < m && a[x] == b[y] { // slide down the diagonal`；L204 `if x >= n && y >= m {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function myers（internal/diff/diff.go:175-210）
- **可信生产调用方（1）：** `Build`（`internal/diff/diff.go:51`，调用点 L63，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `myersMaxD`（`internal/diff/diff.go:150`，调用点 L180，confidence=0.90，exact-match）；`myersVectorLen`（`internal/diff/diff.go:163`，调用点 L182，confidence=0.90，exact-match）；`backtrack`（`internal/diff/diff.go:215`，调用点 L205，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Build`（`internal/diff/diff.go:51`，调用点 L63，confidence=0.90，exact-match，待源码复核）；`deleteSymbol::Execute`（`internal/tool/builtin/delete_symbol.go:57`，调用点 L99，confidence=0.90，import，待源码复核）；`deleteSymbol::Preview`（`internal/tool/builtin/delete_symbol.go:103`，调用点 L141，confidence=0.90，import，待源码复核）；`notebookEdit::Preview`（`internal/tool/builtin/notebookedit.go:113`，调用点 L134，confidence=0.90，import，待源码复核）；`writeFile::Preview`（`internal/tool/builtin/preview.go:26`，调用点 L46，confidence=0.90，import，待源码复核）；`editFile::Preview`（`internal/tool/builtin/preview.go:52`，调用点 L84，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/diff`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/diff/diff_test.go`
- **同目录测试候选：** `internal/diff/diff_extra_test.go`、`internal/diff/largediff_test.go`
- **需同步检查：** `internal/diff/diff.go`、`internal/diff/diff_test.go`、`internal/diff/diff_extra_test.go`、`internal/diff/largediff_test.go`

#### 待修改项

- [ ] 逐段核对 L177、L185、L193、L199、L204 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `myers` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/diff/diff.go|myers` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/diff/diff.go:175-210` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `myers` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/diff/diff_test.go`。
- [ ] 运行相关包测试：`go test ./internal/diff`。
- [ ] 运行相关包 vet：`go vet ./internal/diff`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/diff/diff.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/diff/diff.go|myers` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-259 · 圈复杂度 · `listDefinitions`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-259 |
| Baseline key | `complexity\|internal/expert/store.go\|listDefinitions` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/expert/store.go` |
| 代码行 | 518 |
| 函数/方法/包 | `listDefinitions` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`listDefinitions` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 518-570 行；可见控制流热点：L524 `if err != nil {`；L535 `if err := rows.Scan(`；L546 `if avatar.Valid {`；L552 `if err := decodeColumn(def.ID, "tags_json", tagsJSON, &def.Tags); err != nil {`；L558 `if def.Mode == ModeTeam {`；L566 `if err := rows.Err(); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（decodeColumn、ParseMode、normalizeMemberKinds）、持久化/I-O（ArchiveSink::Err）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function listDefinitions（internal/expert/store.go:518-570）
- **可信生产调用方（1）：** `Store::List`（`internal/expert/store.go:25`，调用点 L35，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `decodeColumn`（`internal/expert/store.go:572`，调用点 L549/L552/L555/L561，confidence=0.90，exact-match，4 条边）；`ParseMode`（`internal/expert/expert.go:106`，调用点 L541，confidence=0.90，exact-match）；`trimMembers`（`internal/expert/store.go:605`，调用点 L559，confidence=0.90，exact-match）；`normalizeMemberKinds`（`internal/expert/store.go:626`，调用点 L559，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L566，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Store::List`（`internal/expert/store.go:25`，调用点 L35，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/expert`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/expert/expert_test.go`、`internal/expert/store_test.go`
- **需同步检查：** `internal/expert/store.go`、`internal/expert/expert.go`、`internal/expert/expert_test.go`、`internal/expert/store_test.go`

#### 待修改项

- [ ] 逐段核对 L524、L542、L549、L558、L566 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `listDefinitions` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/expert/store.go|listDefinitions` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/expert/store.go:518-570` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `listDefinitions` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/expert/expert_test.go`、`internal/expert/store_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/expert`。
- [ ] 运行相关包 vet：`go vet ./internal/expert`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/expert/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/expert/store.go|listDefinitions` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-244 · 圈复杂度 · `unified`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-244 |
| Baseline key | `complexity\|internal/diff/diff.go\|unified` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/diff/diff.go` |
| 代码行 | 266 |
| 函数/方法/包 | `unified` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`unified` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 266-299 行；可见控制流热点：L269 `if len(hunks) == 0 {`；L280 `for i := h.start; i < h.end; i++ {`；L282 `switch r.op.typ {`；L285 `case opDelete:`；L287 `if !oldEOL && r.oldNo == lastOldNo {`；L292 `if !newEOL && r.newNo == lastNewNo {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（writeHunkHeader）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function unified（internal/diff/diff.go:266-299）
- **可信生产调用方（1）：** `Build`（`internal/diff/diff.go:51`，调用点 L80，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `number`（`internal/diff/diff.go:302`，调用点 L267，confidence=0.90，exact-match）；`lastLineNumbers`（`internal/diff/diff.go:327`，调用点 L277，confidence=0.90，exact-match）；`group`（`internal/diff/diff.go:344`，调用点 L268，confidence=0.90，exact-match）；`writeHunkHeader`（`internal/diff/diff.go:375`，调用点 L279，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Build`（`internal/diff/diff.go:51`，调用点 L80，confidence=0.90，exact-match，待源码复核）；`deleteSymbol::Execute`（`internal/tool/builtin/delete_symbol.go:57`，调用点 L99，confidence=0.90，import，待源码复核）；`deleteSymbol::Preview`（`internal/tool/builtin/delete_symbol.go:103`，调用点 L141，confidence=0.90，import，待源码复核）；`notebookEdit::Preview`（`internal/tool/builtin/notebookedit.go:113`，调用点 L134，confidence=0.90，import，待源码复核）；`writeFile::Preview`（`internal/tool/builtin/preview.go:26`，调用点 L46，confidence=0.90，import，待源码复核）；`editFile::Preview`（`internal/tool/builtin/preview.go:52`，调用点 L84，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/diff`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/diff/diff_extra_test.go`、`internal/diff/diff_test.go`、`internal/diff/largediff_test.go`
- **需同步检查：** `internal/diff/diff.go`、`internal/diff/diff_extra_test.go`、`internal/diff/diff_test.go`、`internal/diff/largediff_test.go`

#### 待修改项

- [ ] 逐段核对 L269、L280、L283、L287、L292 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `unified` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/diff/diff.go|unified` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/diff/diff.go:266-299` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `unified` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/diff/diff_extra_test.go`、`internal/diff/diff_test.go`、`internal/diff/largediff_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/diff`。
- [ ] 运行相关包 vet：`go vet ./internal/diff`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/diff/diff.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/diff/diff.go|unified` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-258 · 圈复杂度 · `migrateFirstPrompt`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-258 |
| Baseline key | `complexity\|internal/expert/store.go\|migrateFirstPrompt` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/expert/store.go` |
| 代码行 | 247 |
| 函数/方法/包 | `migrateFirstPrompt` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`migrateFirstPrompt` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 247-274 行；可见控制流热点：L248 `if strings.TrimSpace(id) == "" || oldPrompt == "" || newPrompt == "" || oldPrompt == newPrompt {`；L252 `if err := db.QueryRowContext(ctx, \`SELECT prompts_json FROM experts WHERE id = ?\`, id).Scan(&prompt…`；L253 `if err == sql.ErrNoRows {`；L262 `if len(prompts) == 0 || prompts[0] != oldPrompt {`；L267 `if err != nil {`；L270 `if _, err := db.ExecContext(ctx, \`UPDATE experts SET prompts_json = ?, updated_at = ? WHERE id = ?\`…`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（decodeColumn）、渲染/输出（marshalColumn）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function migrateFirstPrompt（internal/expert/store.go:247-274）
- **可信生产调用方（1）：** `migrateBuiltinPromptUpdates`（`internal/expert/store.go:230`，调用点 L235，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `marshalColumn`（`internal/expert/store.go:510`，调用点 L266，confidence=0.90，exact-match）；`decodeColumn`（`internal/expert/store.go:572`，调用点 L259，confidence=0.90，exact-match）；`nonNilStrings`（`internal/expert/store.go:586`，调用点 L266，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/expert`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/expert/expert_test.go`、`internal/expert/store_test.go`
- **需同步检查：** `internal/expert/store.go`、`internal/expert/expert_test.go`、`internal/expert/store_test.go`

#### 待修改项

- [ ] 逐段核对 L248、L253、L259、L267、L270 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `migrateFirstPrompt` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/expert/store.go|migrateFirstPrompt` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/expert/store.go:247-274` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `migrateFirstPrompt` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/expert/expert_test.go`、`internal/expert/store_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/expert`。
- [ ] 运行相关包 vet：`go vet ./internal/expert`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/expert/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/expert/store.go|migrateFirstPrompt` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-260 · 圈复杂度 · `filterDefinitions`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-260 |
| Baseline key | `complexity\|internal/expert/store.go\|filterDefinitions` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/expert/store.go` |
| 代码行 | 655 |
| 函数/方法/包 | `filterDefinitions` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`filterDefinitions` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 655-678 行；可见控制流热点：L657 `if category == "全部" || strings.EqualFold(category, "all") {`；L661 `if filter.Mode == "" && category == "" && query == "" {`；L665 `for _, def := range defs {`；L666 `if filter.Mode != "" && def.Mode != filter.Mode {`；L669 `if category != "" && !containsFold(def.Categories, category) {`；L672 `if query != "" && !definitionMatchesQuery(def, query) {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function filterDefinitions（internal/expert/store.go:655-678）
- **可信生产调用方（1）：** `Store::ListFiltered`（`internal/expert/store.go:38`，调用点 L43，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `containsFold`（`internal/expert/store.go:680`，调用点 L669，confidence=0.90，exact-match）；`definitionMatchesQuery`（`internal/expert/store.go:689`，调用点 L672，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::ListExpertsByFilter`（`desktop/experts_app.go:65`，调用点 L74，confidence=0.80，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/expert`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/expert/expert_test.go`、`internal/expert/store_test.go`
- **需同步检查：** `internal/expert/store.go`、`internal/expert/expert_test.go`、`internal/expert/store_test.go`

#### 待修改项

- [ ] 逐段核对 L657、L661、L666、L669、L672 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `filterDefinitions` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/expert/store.go|filterDefinitions` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/expert/store.go:655-678` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `filterDefinitions` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/expert/expert_test.go`、`internal/expert/store_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/expert`。
- [ ] 运行相关包 vet：`go vet ./internal/expert`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/expert/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/expert/store.go|filterDefinitions` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-263 · 圈复杂度 · `DetectQuick`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-263 |
| Baseline key | `complexity\|internal/fileutil/encoding/encoding.go\|DetectQuick` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/fileutil/encoding/encoding.go` |
| 代码行 | 77 |
| 函数/方法/包 | `DetectQuick` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`DetectQuick` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 77-87 行；可见控制流热点：L78 `switch {`；L79 `case len(peek) >= 3 && peek[0] == 0xEF && peek[1] == 0xBB && peek[2] == 0xBF:`；L81 `case len(peek) >= 2 && peek[0] == 0xFF && peek[1] == 0xFE:`；L83 `case len(peek) >= 2 && peek[0] == 0xFE && peek[1] == 0xFF:`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function DetectQuick（internal/fileutil/encoding/encoding.go:77-87）
- **可信生产调用方（3）：** `readFile::scanBOMText`（`internal/tool/builtin/readfile.go:160`，调用点 L161/L168，confidence=0.90，import，2 条边）；`App::ReadFile`（`desktop/app.go:6599`，调用点 L6649，confidence=0.90，import）；`grepTool::Execute`（`internal/tool/builtin/grep.go:87`，调用点 L137，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::ReadFile`（`desktop/app.go:6599`，调用点 L6649，confidence=0.90，import，待源码复核）；`grepTool::Execute`（`internal/tool/builtin/grep.go:87`，调用点 L137，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/fileutil`、`internal/tool`、`desktop`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/fileutil/encoding/encoding_test.go`
- **需同步检查：** `internal/fileutil/encoding/encoding.go`、`internal/tool/builtin/readfile.go`、`desktop/app.go`、`internal/tool/builtin/grep.go`、`internal/fileutil/encoding/encoding_test.go`

#### 待修改项

- [ ] 逐段核对 L78、L79、L81、L83 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `DetectQuick` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/fileutil/encoding/encoding.go|DetectQuick` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/fileutil/encoding/encoding.go:77-87` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `DetectQuick` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/fileutil/encoding/encoding_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/fileutil/encoding`。
- [ ] 运行相关包 vet：`go vet ./internal/fileutil/encoding`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/fileutil/encoding/encoding.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/fileutil/encoding/encoding.go|DetectQuick` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-243 · 圈复杂度 · `backtrack`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-243 |
| Baseline key | `complexity\|internal/diff/diff.go\|backtrack` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/diff/diff.go` |
| 代码行 | 215 |
| 函数/方法/包 | `backtrack` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`backtrack` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 215-252 行；可见控制流热点：L218 `for d := len(trace) - 1; d > 0; d-- {`；L222 `if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {`；L230 `for x > prevX && y > prevY { // diagonal: equal lines`；L235 `if x == prevX {`；L243 `for x > 0 && y > 0 {`；L248 `for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function backtrack（internal/diff/diff.go:215-252）
- **可信生产调用方（1）：** `myers`（`internal/diff/diff.go:175`，调用点 L205，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Build`（`internal/diff/diff.go:51`，调用点 L63，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/diff`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/diff/diff_extra_test.go`、`internal/diff/diff_test.go`、`internal/diff/largediff_test.go`
- **需同步检查：** `internal/diff/diff.go`、`internal/diff/diff_extra_test.go`、`internal/diff/diff_test.go`、`internal/diff/largediff_test.go`

#### 待修改项

- [ ] 逐段核对 L218、L222、L235、L243、L248 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `backtrack` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/diff/diff.go|backtrack` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/diff/diff.go:215-252` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `backtrack` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/diff/diff_extra_test.go`、`internal/diff/diff_test.go`、`internal/diff/largediff_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/diff`。
- [ ] 运行相关包 vet：`go vet ./internal/diff`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/diff/diff.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/diff/diff.go|backtrack` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-257 · 圈复杂度 · `definitionSystemPrompt`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-257 |
| Baseline key | `complexity\|internal/expert/definition_catalog.go\|definitionSystemPrompt` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 32） |
| 文件 | `internal/expert/definition_catalog.go` |
| 代码行 | 75 |
| 函数/方法/包 | `definitionSystemPrompt` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`definitionSystemPrompt` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 75-123 行；可见控制流热点：L78 `if def.Role != "" {`；L84 `if def.Mode == ModeTeam {`；L92 `for _, member := range def.Members {`；L101 `if member.Task != "" {`；L112 `if def.SystemPrompt != "" {`；L118 `for _, prompt := range def.Prompts {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function definitionSystemPrompt（internal/expert/definition_catalog.go:75-123）
- **可信生产调用方（1）：** `expertFromDefinition`（`internal/expert/definition_catalog.go:51`，调用点 L65，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/expert`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/expert/expert_test.go`、`internal/expert/store_test.go`
- **需同步检查：** `internal/expert/definition_catalog.go`、`internal/expert/expert_test.go`、`internal/expert/store_test.go`

#### 待修改项

- [ ] 逐段核对 L78、L90、L98、L104、L118 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `definitionSystemPrompt` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/expert/definition_catalog.go|definitionSystemPrompt` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/expert/definition_catalog.go:75-123` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `definitionSystemPrompt` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/expert/expert_test.go`、`internal/expert/store_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/expert`。
- [ ] 运行相关包 vet：`go vet ./internal/expert`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/expert/definition_catalog.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/expert/definition_catalog.go|definitionSystemPrompt` 下降或消失且无新增/恶化。
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
