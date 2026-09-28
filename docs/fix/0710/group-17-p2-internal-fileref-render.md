# G17 · internal/fileref · render 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G17 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/fileref`、`internal/bot` |
| 涉及文件 | `internal/fileref/search.go`<br>`internal/bot/render.go`<br>`internal/bot/qq/gateway.go`<br>`internal/bot/weixin/weixin_login.go` |
| 主要治理目标 | 围绕 `internal/bot/render.go` 等治理对象降低复杂度/体量，统一处理 3 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | fileref / bot 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 低：仍需开始前复查开放 PR |
| 冲突依据 | 未发现静态热点或本地远端引用重叠 |
| 建议处理顺序 | 总序第 17 组；无同文件组依赖 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 返回值、错误语义和现有调用顺序；权限、安全或凭据边界；并发、取消或事件顺序；持久化、迁移或文件格式兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **51/100**，对象 `Search` 的分项为严重度 13.8、集中度 8.6、影响面 8.7、可执行性 20.0；最高超限 3.00x，文件内 2 条问题，CodeGraph 记录 2 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数，必须合并治理。

### 治理边界与组间关系

本组涉及 4 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-261` | `complexity\|internal/fileref/search.go\|Search` | 圈复杂度 | `internal/fileref/search.go` | `Search` | 53 | 30/10 | +20（3.00x） | P2 | 2 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `FN-036` | `func_lines\|internal/fileref/search.go\|Search` | 函数有效行数 | `internal/fileref/search.go` | `Search` | 53 | 86/80 | +6（1.07x） | P2 | 2 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-090` | `complexity\|internal/bot/render.go\|*renderSink.Emit` | 圈复杂度 | `internal/bot/render.go` | `*renderSink.Emit` | 65 | 25/10 | +15（2.50x） | P2 | 0 可信 + 0 待核 caller / 5 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `FN-014` | `func_lines\|internal/bot/render.go\|*renderSink.Emit` | 函数有效行数 | `internal/bot/render.go` | `*renderSink.Emit` | 65 | 105/80 | +25（1.31x） | P2 | 0 可信 + 0 待核 caller / 5 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-089` | `complexity\|internal/bot/qq/gateway.go\|*adapter.connectGateway` | 圈复杂度 | `internal/bot/qq/gateway.go` | `*adapter.connectGateway` | 214 | 24/10 | +14（2.40x） | P2 | 0 可信 + 1 待核 caller / 1 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `FN-013` | `func_lines\|internal/bot/qq/gateway.go\|*adapter.connectGateway` | 函数有效行数 | `internal/bot/qq/gateway.go` | `*adapter.connectGateway` | 214 | 119/80 | +39（1.49x） | P2 | 0 可信 + 1 待核 caller / 1 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-093` | `complexity\|internal/bot/weixin/weixin_login.go\|PollLoginWithPaths` | 圈复杂度 | `internal/bot/weixin/weixin_login.go` | `PollLoginWithPaths` | 223 | 20/10 | +10（2.00x） | P2 | 3 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-092` | `complexity\|internal/bot/weixin/weixin_login.go\|LoginWithPaths` | 圈复杂度 | `internal/bot/weixin/weixin_login.go` | `LoginWithPaths` | 149 | 14/10 | +4（1.40x） | P3 | 2 可信 + 0 待核 caller / 2 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-088` | `complexity\|internal/bot/qq/gateway.go\|*adapter.getAccessToken` | 圈复杂度 | `internal/bot/qq/gateway.go` | `*adapter.getAccessToken` | 121 | 14/10 | +4（1.40x） | P3 | 0 可信 + 3 待核 caller / 1 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-091` | `complexity\|internal/bot/render.go\|lastSemanticBoundary` | 圈复杂度 | `internal/bot/render.go` | `lastSemanticBoundary` | 286 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-261 · 圈复杂度 · `Search`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-261 |
| Baseline key | `complexity\|internal/fileref/search.go\|Search` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 51） |
| 文件 | `internal/fileref/search.go` |
| 代码行 | 53 |
| 函数/方法/包 | `Search` |
| Baseline / 当前复测 / 阈值 | 30 / 30 / 10 |
| 超出 | +20，3.00x |
| 同一治理对象关联问题 | `FN-036` |

#### 明确的问题描述

`Search` 的圈复杂度当前为 **30**，超过限制 **10**（+20，3.00x）。定义范围为 53-148 行；可见控制流热点：L55 `if len(query) < minQueryLen || strings.ContainsAny(query, \`/\\`) || limit <= 0 {`；L75 `if visited > maxWalkEntries {`；L91 `if strings.Contains(strings.ToLower(name), query) {`；L106 `if err != nil {`；L129 `if nDirs > dirQuota {`；L142 `if len(segmentHits) > remaining {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function Search（internal/fileref/search.go:53-148）
- **可信生产调用方（2）：** `App::SearchFileRefs`（`desktop/app.go:6585`，调用点 L6590，confidence=0.90，import）；`chatTUI::searchFileRefs`（`internal/cli/complete.go:486`，调用点 L499，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `pathSegmentContains`（`internal/fileref/search.go:155`，调用点 L114，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::SearchFileRefs`（`desktop/app.go:6585`，调用点 L6590，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/fileref`、`desktop`、`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/fileref/search_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/fileref/search.go`、`desktop/app.go`、`internal/cli/complete.go`、`internal/fileref/search_test.go`

#### 待修改项

- [ ] 逐段核对 L55、L80、L99、L114、L142 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `Search` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-261、FN-036 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/fileref/search.go|Search` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/fileref/search.go:53-148` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Search` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-261`、`FN-036` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/fileref/search_test.go`。
- [ ] 运行相关包测试：`go test ./internal/fileref`。
- [ ] 运行相关包 vet：`go vet ./internal/fileref`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/fileref/search.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/fileref/search.go|Search` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-036 · 函数有效行数 · `Search`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-036 |
| Baseline key | `func_lines\|internal/fileref/search.go\|Search` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 51） |
| 文件 | `internal/fileref/search.go` |
| 代码行 | 53 |
| 函数/方法/包 | `Search` |
| Baseline / 当前复测 / 阈值 | 86 / 86 / 80 |
| 超出 | +6，1.07x |
| 同一治理对象关联问题 | `CX-261` |

#### 明确的问题描述

`Search` 的有效代码行 **86**，超过限制 **80**（+6）。CodeGraph 定义物理范围为 53-148 行，并记录 1 个直接下游候选；候选职责分布为 CodeGraph 未提供足够的下游调用分类。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function Search（internal/fileref/search.go:53-148）
- **可信生产调用方（2）：** `App::SearchFileRefs`（`desktop/app.go:6585`，调用点 L6590，confidence=0.90，import）；`chatTUI::searchFileRefs`（`internal/cli/complete.go:486`，调用点 L499，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `pathSegmentContains`（`internal/fileref/search.go:155`，调用点 L114，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::SearchFileRefs`（`desktop/app.go:6585`，调用点 L6590，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/fileref`、`desktop`、`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/fileref/search_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/fileref/search.go`、`desktop/app.go`、`internal/cli/complete.go`、`internal/fileref/search_test.go`

#### 待修改项

- [ ] 把 `Search` 的 86 个有效行按 输入、处理、输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L89 `Allow matching directory names so the user can select a`、L122 `Directories first so the user can navigate into them; then basename`、L123 `hits (most relevant file matches); then path-segment hits. We reserve`、L125 `out by a large number of file matches.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-261、FN-036 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/fileref/search.go|Search` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/fileref/search.go:53-148` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Search` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-261`、`FN-036` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/fileref/search_test.go`。
- [ ] 运行相关包测试：`go test ./internal/fileref`。
- [ ] 运行相关包 vet：`go vet ./internal/fileref`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/fileref/search.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/fileref/search.go|Search` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-090 · 圈复杂度 · `*renderSink.Emit`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-090 |
| Baseline key | `complexity\|internal/bot/render.go\|*renderSink.Emit` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 50） |
| 文件 | `internal/bot/render.go` |
| 代码行 | 65 |
| 函数/方法/包 | `*renderSink.Emit` |
| Baseline / 当前复测 / 阈值 | 25 / 25 / 10 |
| 超出 | +15，2.50x |
| 同一治理对象关联问题 | `FN-014` |

#### 明确的问题描述

`*renderSink.Emit` 的圈复杂度当前为 **25**，超过限制 **10**（+15，2.50x）。定义范围为 65-188 行；可见控制流热点：L66 `switch e.Kind {`；L82 `if s.inThinking {`；L100 `if e.Tool.Err != "" {`；L124 `case PlatformQQ:`；L150 `case event.TurnDone:`；L178 `case event.CompactionStarted:`。CodeGraph 记录的下游职责候选主要涉及 渲染/输出（renderToolName、renderAskText、renderSink::sendProgress）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method renderSink::Emit（internal/bot/render.go:65-188）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `renderToolName`（`internal/bot/render.go:254`，调用点 L91/L98，confidence=0.90，exact-match，2 条边）；`approvalKeyboard`（`internal/bot/render.go:353`，调用点 L125，confidence=0.90，exact-match）；`approvalCard`（`internal/bot/render.go:362`，调用点 L127，confidence=0.90，exact-match）；`renderAskText`（`internal/bot/render.go:388`，调用点 L136，confidence=0.90，exact-match）；`askCard`（`internal/bot/render.go:413`，调用点 L146，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `adapter::Platform`（`internal/bot/feishu/feishu.go:106`，调用点 L123/L145，confidence=0.70，exact-match，2 条边，待源码复核）；`renderSink::sendProgress`（`internal/bot/render.go:230`，调用点 L93/L101，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/gateway_test.go`、`internal/bot/qq/gateway_test.go`、`internal/bot/render_test.go`、`internal/bot/session_test.go`、`internal/bot/turn_dispatch_test.go`、`internal/bot/weixin/weixin_test.go`
- **需同步检查：** `internal/bot/render.go`、`internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/gateway_test.go`

#### 待修改项

- [ ] 逐段核对 L66、L87、L110、L145、L178 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*renderSink.Emit` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-090、FN-014 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/bot/render.go|*renderSink.Emit` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/render.go:65-188` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*renderSink.Emit` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-090`、`FN-014` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/gateway_test.go`、`internal/bot/qq/gateway_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/bot`。
- [ ] 运行相关包 vet：`go vet ./internal/bot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/render.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/bot/render.go|*renderSink.Emit` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-014 · 函数有效行数 · `*renderSink.Emit`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-014 |
| Baseline key | `func_lines\|internal/bot/render.go\|*renderSink.Emit` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 50） |
| 文件 | `internal/bot/render.go` |
| 代码行 | 65 |
| 函数/方法/包 | `*renderSink.Emit` |
| Baseline / 当前复测 / 阈值 | 105 / 105 / 80 |
| 超出 | +25，1.31x |
| 同一治理对象关联问题 | `CX-090` |

#### 明确的问题描述

`*renderSink.Emit` 的有效代码行 **105**，超过限制 **80**（+25）。CodeGraph 定义物理范围为 65-188 行，并记录 7 个直接下游候选；候选职责分布为 渲染/输出（renderToolName、renderAskText、renderSink::sendProgress）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method renderSink::Emit（internal/bot/render.go:65-188）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `renderToolName`（`internal/bot/render.go:254`，调用点 L91/L98，confidence=0.90，exact-match，2 条边）；`approvalKeyboard`（`internal/bot/render.go:353`，调用点 L125，confidence=0.90，exact-match）；`approvalCard`（`internal/bot/render.go:362`，调用点 L127，confidence=0.90，exact-match）；`renderAskText`（`internal/bot/render.go:388`，调用点 L136，confidence=0.90，exact-match）；`askCard`（`internal/bot/render.go:413`，调用点 L146，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `adapter::Platform`（`internal/bot/feishu/feishu.go:106`，调用点 L123/L145，confidence=0.70，exact-match，2 条边，待源码复核）；`renderSink::sendProgress`（`internal/bot/render.go:230`，调用点 L93/L101，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/gateway_test.go`、`internal/bot/qq/gateway_test.go`、`internal/bot/render_test.go`、`internal/bot/session_test.go`、`internal/bot/turn_dispatch_test.go`、`internal/bot/weixin/weixin_test.go`
- **需同步检查：** `internal/bot/render.go`、`internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/gateway_test.go`

#### 待修改项

- [ ] 把 `*renderSink.Emit` 的 105 个有效行按 渲染/输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L88 `full message received, do nothing extra`、L106 `still records the complete controller turn for desktop review.`、L109 `发送审批请求`、L151 `刷新缓冲` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-090、FN-014 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/bot/render.go|*renderSink.Emit` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/render.go:65-188` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*renderSink.Emit` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-090`、`FN-014` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/gateway_test.go`、`internal/bot/qq/gateway_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/bot`。
- [ ] 运行相关包 vet：`go vet ./internal/bot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/render.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/bot/render.go|*renderSink.Emit` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-089 · 圈复杂度 · `*adapter.connectGateway`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-089 |
| Baseline key | `complexity\|internal/bot/qq/gateway.go\|*adapter.connectGateway` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 49） |
| 文件 | `internal/bot/qq/gateway.go` |
| 代码行 | 214 |
| 函数/方法/包 | `*adapter.connectGateway` |
| Baseline / 当前复测 / 阈值 | 24 / 24 / 10 |
| 超出 | +14，2.40x |
| 同一治理对象关联问题 | `FN-013` |

#### 明确的问题描述

`*adapter.connectGateway` 的圈复杂度当前为 **24**，超过限制 **10**（+14，2.40x）。定义范围为 214-348 行；可见控制流热点：L216 `if err != nil {`；L247 `if msg.Op != opHello {`；L276 `if msg.Op == opDispatch && msg.T == "READY" {`；L303 `case <-ticker.C:`；L321 `if err := decoder.Decode(&msg); err != nil {`；L339 `case opInvalid:`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（adapter::handleDispatch）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method adapter::connectGateway（internal/bot/qq/gateway.go:214-348）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `adapter::gatewayLoop`（`internal/bot/qq/gateway.go:108`，调用点 L117，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `sanitizeHeartbeatInterval`（`internal/bot/qq/gateway.go:430`，调用点 L254，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `adapter::appID`（`internal/bot/qq/gateway.go:350`，调用点 L228，confidence=0.70，instance-method，待源码复核）；`adapter::getGatewayURL`（`internal/bot/qq/gateway.go:378`，调用点 L215，confidence=0.70，instance-method，待源码复核）；`adapter::handleDispatch`（`internal/bot/qq/gateway.go:450`，调用点 L332，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `adapter::Start`（`internal/bot/qq/adapter.go:50`，调用点 L63，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/bot/qq/gateway_test.go`
- **需同步检查：** `internal/bot/qq/gateway.go`、`internal/bot/qq/gateway_test.go`

#### 待修改项

- [ ] 逐段核对 L216、L251、L300、L320、L339 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*adapter.connectGateway` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-089、FN-013 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/bot/qq/gateway.go|*adapter.connectGateway` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/qq/gateway.go:214-348` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*adapter.connectGateway` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-089`、`FN-013` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/bot/qq/gateway_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/bot/qq`。
- [ ] 运行相关包 race：`go test -race ./internal/bot/qq`。
- [ ] 运行相关包 vet：`go vet ./internal/bot/qq`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/qq/gateway.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/bot/qq/gateway.go|*adapter.connectGateway` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-013 · 函数有效行数 · `*adapter.connectGateway`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-013 |
| Baseline key | `func_lines\|internal/bot/qq/gateway.go\|*adapter.connectGateway` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 49） |
| 文件 | `internal/bot/qq/gateway.go` |
| 代码行 | 214 |
| 函数/方法/包 | `*adapter.connectGateway` |
| Baseline / 当前复测 / 阈值 | 119 / 119 / 80 |
| 超出 | +39，1.49x |
| 同一治理对象关联问题 | `CX-089` |

#### 明确的问题描述

`*adapter.connectGateway` 的有效代码行 **119**，超过限制 **80**（+39）。CodeGraph 定义物理范围为 214-348 行，并记录 4 个直接下游候选；候选职责分布为 状态与控制（adapter::handleDispatch）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method adapter::connectGateway（internal/bot/qq/gateway.go:214-348）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `adapter::gatewayLoop`（`internal/bot/qq/gateway.go:108`，调用点 L117，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `sanitizeHeartbeatInterval`（`internal/bot/qq/gateway.go:430`，调用点 L254，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `adapter::appID`（`internal/bot/qq/gateway.go:350`，调用点 L228，confidence=0.70，instance-method，待源码复核）；`adapter::getGatewayURL`（`internal/bot/qq/gateway.go:378`，调用点 L215，confidence=0.70，instance-method，待源码复核）；`adapter::handleDispatch`（`internal/bot/qq/gateway.go:450`，调用点 L332，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `adapter::Start`（`internal/bot/qq/adapter.go:50`，调用点 L63，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/bot/qq/gateway_test.go`
- **需同步检查：** `internal/bot/qq/gateway.go`、`internal/bot/qq/gateway_test.go`

#### 待修改项

- [ ] 把 `*adapter.connectGateway` 的 119 个有效行按 状态与控制 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L243 `第一次读取必须是 Hello`、L256 `Identify`、L291 `启动 heartbeat`、L319 `主循环：读取 dispatch 事件` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-089、FN-013 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/bot/qq/gateway.go|*adapter.connectGateway` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/qq/gateway.go:214-348` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*adapter.connectGateway` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-089`、`FN-013` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/bot/qq/gateway_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/bot/qq`。
- [ ] 运行相关包 race：`go test -race ./internal/bot/qq`。
- [ ] 运行相关包 vet：`go vet ./internal/bot/qq`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/qq/gateway.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/bot/qq/gateway.go|*adapter.connectGateway` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-093 · 圈复杂度 · `PollLoginWithPaths`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-093 |
| Baseline key | `complexity\|internal/bot/weixin/weixin_login.go\|PollLoginWithPaths` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 45） |
| 文件 | `internal/bot/weixin/weixin_login.go` |
| 代码行 | 223 |
| 函数/方法/包 | `PollLoginWithPaths` |
| Baseline / 当前复测 / 阈值 | 20 / 20 / 10 |
| 超出 | +10，2.00x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`PollLoginWithPaths` 的圈复杂度当前为 **20**，超过限制 **10**（+10，2.00x）。定义范围为 223-275 行；可见控制流热点：L224 `if session == nil || session.QRCode == "" {`；L236 `switch status {`；L239 `case "scaned":`；L246 `case "confirmed":`；L254 `if accountID == "" || accountID == "<nil>" || token == "" || token == "<nil>" {`；L270 `case "expired":`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（saveAccountWithPaths）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function PollLoginWithPaths（internal/bot/weixin/weixin_login.go:223-275）
- **可信生产调用方（3）：** `App::PollBotConnectionInstall`（`desktop/bot_connection_app.go:142`，调用点 L158，confidence=0.90，import）；`LoginWithPaths`（`internal/bot/weixin/weixin_login.go:149`，调用点 L173，confidence=0.90，exact-match）；`PollLogin`（`internal/bot/weixin/weixin_login.go:219`，调用点 L220，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `saveAccountWithPaths`（`internal/bot/weixin/weixin_login.go:129`，调用点 L263/L266，confidence=0.90，exact-match，2 条边）；`ilinkGET`（`internal/bot/weixin/weixin.go:296`，调用点 L231，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::PollBotConnectionInstall`（`desktop/bot_connection_app.go:142`，调用点 L158，confidence=0.90，import，待源码复核）；`Login`（`internal/bot/weixin/weixin_login.go:145`，调用点 L146，confidence=0.90，exact-match，待源码复核）；`PollLogin`（`internal/bot/weixin/weixin_login.go:219`，调用点 L220，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/bot`、`desktop`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/bot/weixin/weixin_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/bot/weixin/weixin_login.go`、`desktop/bot_connection_app.go`、`internal/bot/weixin/weixin.go`、`internal/bot/weixin/weixin_test.go`

#### 待修改项

- [ ] 逐段核对 L224、L236、L242、L254、L270 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `PollLoginWithPaths` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/bot/weixin/weixin_login.go|PollLoginWithPaths` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/weixin/weixin_login.go:223-275` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `PollLoginWithPaths` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/bot/weixin/weixin_test.go`。
- [ ] 运行相关包测试：`go test ./internal/bot/weixin`。
- [ ] 运行相关包 vet：`go vet ./internal/bot/weixin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/weixin/weixin_login.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/bot/weixin/weixin_login.go|PollLoginWithPaths` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-092 · 圈复杂度 · `LoginWithPaths`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-092 |
| Baseline key | `complexity\|internal/bot/weixin/weixin_login.go\|LoginWithPaths` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/bot/weixin/weixin_login.go` |
| 代码行 | 149 |
| 函数/方法/包 | `LoginWithPaths` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`LoginWithPaths` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 149-195 行；可见控制流热点：L150 `if timeout <= 0 {`；L159 `if session.QRCodeURL != "" {`；L169 `case <-ctx.Done():`；L174 `if err != nil {`；L183 `if out != nil {`；L187 `case "scaned":`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（StartLogin）、持久化/I-O（ArchiveSink::Err）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function LoginWithPaths（internal/bot/weixin/weixin_login.go:149-195）
- **可信生产调用方（2）：** `Login`（`internal/bot/weixin/weixin_login.go:145`，调用点 L146，confidence=0.90，exact-match）；`botWeixinLogin`（`internal/cli/bot.go:319`，调用点 L341，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `StartLogin`（`internal/bot/weixin/weixin_login.go:197`，调用点 L153，confidence=0.90，exact-match）；`PollLoginWithPaths`（`internal/bot/weixin/weixin_login.go:223`，调用点 L173，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L170，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Login`（`internal/bot/weixin/weixin_login.go:145`，调用点 L146，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/bot`、`internal/cli`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/bot/weixin/weixin_test.go`
- **需同步检查：** `internal/bot/weixin/weixin_login.go`、`internal/cli/bot.go`、`internal/automation/archive_sink.go`、`internal/bot/weixin/weixin_test.go`

#### 待修改项

- [ ] 逐段核对 L150、L167、L171、L183、L187 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `LoginWithPaths` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/bot/weixin/weixin_login.go|LoginWithPaths` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/weixin/weixin_login.go:149-195` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `LoginWithPaths` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/bot/weixin/weixin_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/bot/weixin`。
- [ ] 运行相关包 race：`go test -race ./internal/bot/weixin`。
- [ ] 运行相关包 vet：`go vet ./internal/bot/weixin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/weixin/weixin_login.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/bot/weixin/weixin_login.go|LoginWithPaths` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-088 · 圈复杂度 · `*adapter.getAccessToken`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-088 |
| Baseline key | `complexity\|internal/bot/qq/gateway.go\|*adapter.getAccessToken` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/bot/qq/gateway.go` |
| 代码行 | 121 |
| 函数/方法/包 | `*adapter.getAccessToken` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*adapter.getAccessToken` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 121-191 行；可见控制流热点：L123 `if a.token != "" && time.Now().Before(a.tokenExpiry) {`；L135 `if appSecret == "" {`；L147 `if err != nil {`；L161 `if resp.StatusCode >= 400 {`；L174 `if err != nil {`；L183 `if expiresIn > 60 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method adapter::getAccessToken（internal/bot/qq/gateway.go:121-191）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（3）：** `adapter::Start`（`internal/bot/qq/adapter.go:50`，调用点 L54，confidence=0.70，instance-method，待源码复核）；`adapter::gatewayLoop`（`internal/bot/qq/gateway.go:108`，调用点 L110，confidence=0.70，instance-method，待源码复核）；`adapter::sendMessagePayload`（`internal/bot/qq/gateway.go:585`，调用点 L586，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `qqExpiresInSeconds`（`internal/bot/qq/gateway.go:193`，调用点 L173，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `adapter::appID`（`internal/bot/qq/gateway.go:350`，调用点 L130，confidence=0.70，instance-method，待源码复核）；`adapter::appSecretEnvName`（`internal/bot/qq/gateway.go:357`，调用点 L136，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `adapter::Start`（`internal/bot/qq/adapter.go:50`，调用点 L54，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/bot/qq/gateway_test.go`
- **需同步检查：** `internal/bot/qq/gateway.go`、`internal/bot/qq/adapter.go`、`internal/bot/qq/gateway_test.go`

#### 待修改项

- [ ] 逐段核对 L123、L142、L158、L170、L183 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `*adapter.getAccessToken` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/bot/qq/gateway.go|*adapter.getAccessToken` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/qq/gateway.go:121-191` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*adapter.getAccessToken` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/bot/qq/gateway_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/bot/qq`。
- [ ] 运行相关包 vet：`go vet ./internal/bot/qq`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/qq/gateway.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/bot/qq/gateway.go|*adapter.getAccessToken` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-091 · 圈复杂度 · `lastSemanticBoundary`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-091 |
| Baseline key | `complexity\|internal/bot/render.go\|lastSemanticBoundary` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/bot/render.go` |
| 代码行 | 286 |
| 函数/方法/包 | `lastSemanticBoundary` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`lastSemanticBoundary` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 286-323 行；可见控制流热点：L287 `if maxRunes <= 0 {`；L295 `if strings.HasPrefix(text[idx:], "\`\`\`") {`；L299 `if count > maxRunes {`；L308 `if unicode.IsSpace(r) {`；L309 `if lastNonSpaceBoundary > 0 {`；L317 `if isSemanticBoundaryRune(r) {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（isSemanticBoundaryRune）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function lastSemanticBoundary（internal/bot/render.go:286-323）
- **可信生产调用方（1）：** `renderFlushIndex`（`internal/bot/render.go:264`，调用点 L270/L276/L283，confidence=0.90，exact-match，3 条边）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `isSemanticBoundaryRune`（`internal/bot/render.go:325`，调用点 L317，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `renderSink::flush`（`internal/bot/render.go:190`，调用点 L192，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/gateway_test.go`、`internal/bot/qq/gateway_test.go`、`internal/bot/render_test.go`、`internal/bot/session_test.go`、`internal/bot/turn_dispatch_test.go`、`internal/bot/weixin/weixin_test.go`
- **需同步检查：** `internal/bot/render.go`、`internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/gateway_test.go`

#### 待修改项

- [ ] 逐段核对 L287、L295、L303、L309、L317 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `lastSemanticBoundary` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/bot/render.go|lastSemanticBoundary` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/render.go:286-323` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `lastSemanticBoundary` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/gateway_test.go`、`internal/bot/qq/gateway_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/bot`。
- [ ] 运行相关包 vet：`go vet ./internal/bot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/render.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/bot/render.go|lastSemanticBoundary` 下降或消失且无新增/恶化。
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
