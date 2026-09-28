# G38 · internal/serve · serve 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G38 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/serve`、`internal/skill` |
| 涉及文件 | `internal/serve/serve.go`<br>`internal/skill/skill.go`<br>`internal/serve/auth.go`<br>`internal/skill/tools.go` |
| 主要治理目标 | 围绕 `internal/serve/serve.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | serve / skill 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` |
| 建议处理顺序 | 总序第 38 组；无同文件组依赖 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **41/100**，对象 `internal/serve/serve.go` 的分项为严重度 7.8、集中度 5.3、影响面 16.0、可执行性 12.0；最高超限 1.56x，文件内 4 条问题，CodeGraph 记录 2 个可信生产调用方、4 个待核候选。

### 治理边界与组间关系

本组涉及 4 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `FL-018` | `file_lines\|internal/serve/serve.go\|` | 文件有效行数 | `internal/serve/serve.go` | `internal/serve/serve.go` | 1 | 1245/800 | +445（1.56x） | P3 | 2 可信 + 4 待核 caller / 24 可信 + 15 待核 callee / 297 unresolved | 待修改 |
| `CX-356` | `complexity\|internal/skill/skill.go\|New` | 圈复杂度 | `internal/skill/skill.go` | `New` | 107 | 11/10 | +1（1.10x） | P3 | 7 可信 + 0 待核 caller / 6 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-352` | `complexity\|internal/serve/auth.go\|safeRedirectTarget` | 圈复杂度 | `internal/serve/auth.go` | `safeRedirectTarget` | 471 | 18/10 | +8（1.80x） | P3 | 3 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-355` | `complexity\|internal/serve/serve.go\|*Server.deleteSession` | 圈复杂度 | `internal/serve/serve.go` | `*Server.deleteSession` | 1052 | 18/10 | +8（1.80x） | P3 | 0 可信 + 0 待核 caller / 6 可信 + 2 待核 callee / 22 unresolved | 待修改 |
| `CX-358` | `complexity\|internal/skill/skill.go\|*Store.readEntry` | 圈复杂度 | `internal/skill/skill.go` | `*Store.readEntry` | 409 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-360` | `complexity\|internal/skill/tools.go\|*installSkillTool.Execute` | 圈复杂度 | `internal/skill/tools.go` | `*installSkillTool.Execute` | 445 | 13/10 | +3（1.30x） | P3 | 0 可信 + 0 待核 caller / 3 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-357` | `complexity\|internal/skill/skill.go\|*Store.List` | 圈复杂度 | `internal/skill/skill.go` | `*Store.List` | 289 | 11/10 | +1（1.10x） | P3 | 0 可信 + 0 待核 caller / 1 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-359` | `complexity\|internal/skill/tools.go\|*runSkillTool.Execute` | 圈复杂度 | `internal/skill/tools.go` | `*runSkillTool.Execute` | 76 | 11/10 | +1（1.10x） | P3 | 0 可信 + 0 待核 caller / 3 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-354` | `complexity\|internal/serve/serve.go\|*Server.generateTitle` | 圈复杂度 | `internal/serve/serve.go` | `*Server.generateTitle` | 963 | 17/10 | +7（1.70x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 1 待核 callee / 11 unresolved | 待修改 |
| `CX-353` | `complexity\|internal/serve/serve.go\|*Server.resume` | 圈复杂度 | `internal/serve/serve.go` | `*Server.resume` | 798 | 11/10 | +1（1.10x） | P3 | 0 可信 + 0 待核 caller / 5 可信 + 1 待核 callee / 11 unresolved | 待修改 |

## 3. 逐条治理任务

### FL-018 · 文件有效行数 · `internal/serve/serve.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-018 |
| Baseline key | `file_lines\|internal/serve/serve.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 41） |
| 文件 | `internal/serve/serve.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/serve/serve.go` |
| Baseline / 当前复测 / 阈值 | 1245 / 1245 / 800 |
| 超出 | +445，1.56x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/serve/serve.go` 当前有效代码行 **1245**，超过限制 **800**（+445）。CodeGraph 在文件中索引到的主要大符号包括 Server::deleteSession@1052-1110、Server::handler@265-313、Server::events@367-414、Server::resume@798-845、Server::sessions@1005-1049、Server::switchModel@147-187；文件同时承载的调用影响覆盖 29 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 81 个函数/方法节点。
- **可信生产调用方（2）：** `chatTUI::handleApprovalKey`（`internal/cli/chat_tui.go:2129`，调用点 L2141/L2143/L2145/L2149，confidence=0.90，exact-match，7 条边）；`runServe`（`internal/cli/cli.go:508`，调用点 L611/L616/L614/L617，confidence=0.90，import，4 条边）
- **待源码复核调用方候选（4）：** `Server::RunGraceful`（`internal/serve/run.go:24`，调用点 L28/L45/L31，confidence=0.70，instance-method，3 条边，待源码复核）；`Server::Run`（`internal/serve/run.go:13`，调用点 L17/L18，confidence=0.70，instance-method，2 条边，待源码复核）；`writeAutomationJSON`（`internal/serve/automation.go:118`，调用点 L127，confidence=0.70，exact-match，待源码复核）；`Server::validateListenAuth`（`internal/serve/listen_auth.go:9`，调用点 L10，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（24）：** `decodeJSONRequest`（`internal/serve/request_json.go:11`，调用点 L1056/L802/L422/L1271，confidence=0.90，exact-match，20 条边）；`writeJSONRequestError`（`internal/serve/request_json.go:15`，调用点 L1057/L803/L423/L1272，confidence=0.90，exact-match，20 条边）；`LoadForRootWithActor`（`internal/config/load.go:34`，调用点 L199/L121/L346，confidence=0.90，import，3 条边）；`IsCleanupPending`（`internal/agent/save.go:256`，调用点 L830/L1030，confidence=0.90，import，2 条边）；`SteerText`（`internal/agent/agent.go:487`，调用点 L532，confidence=0.90，import）；`BranchID`（`internal/agent/branch.go:76`，调用点 L1156，confidence=0.90，import）；`UserPreviewText`（`internal/agent/preview.go:110`，调用点 L1215，confidence=0.90，import）；`MarkCleanupPending`（`internal/agent/save.go:227`，调用点 L1096，confidence=0.90，import）
- **待源码复核被调候选（15）：** `Turn::Finish`（`internal/memorycompiler/runtime.go:1728`，调用点 L1116/L1121/L1134，confidence=0.70，instance-method，3 条边，待源码复核）；`Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L204/L125，confidence=0.70，instance-method，2 条边，待源码复核）；`RuntimeIdentity::ActorContext`（`internal/control/port.go:51`，调用点 L108/L110，confidence=0.80，framework，2 条边，待源码复核）；`TeardownResult::HasTimedOut`（`internal/jobs/jobs.go:77`，调用点 L1095/L1115，confidence=0.80，instance-method，2 条边，待源码复核）；`Balance::Display`（`internal/billing/balance.go:116`，调用点 L946，confidence=0.70，instance-method，待源码复核）；`ProviderEntry::APIKey`（`internal/config/config.go:1390`，调用点 L133，confidence=0.70，instance-method，待源码复核）；`Config::DesktopLanguage`（`internal/config/config.go:190`，调用点 L347，confidence=0.70，instance-method，待源码复核）；`Config::Provider`（`internal/config/config.go:1302`，调用点 L237，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 297 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 29 个文件；`internal/serve`、`internal/cli`、`internal/config`、`internal/memorycompiler`、`internal/agent`、`internal/control`、`internal/jobs`、`internal/billing`、`internal/boot`、`internal/fileutil`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/serve/modelswitch_test.go`、`internal/serve/serve_test.go`
- **同目录测试候选：** `internal/serve/actor_test.go`、`internal/serve/auth_test.go`、`internal/serve/automation_test.go`、`internal/serve/broadcaster_test.go`、`internal/serve/csrf_test.go`、`internal/serve/effort_test.go`、`internal/serve/titlecache_test.go`
- **需同步检查：** `internal/serve/serve.go`、`internal/cli/chat_tui.go`、`internal/cli/cli.go`、`internal/serve/run.go`、`internal/serve/request_json.go`、`internal/config/load.go`、`internal/memorycompiler/runtime.go`、`internal/serve/modelswitch_test.go`、`internal/serve/serve_test.go`、`internal/serve/actor_test.go`、`internal/serve/auth_test.go`

#### 待修改项

- [ ] 先冻结 `internal/serve/serve.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `Server::deleteSession`、`Server::handler`、`Server::events`、`Server::resume`、`Server::sessions` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-353、CX-354、CX-355。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/serve/serve.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/serve/serve.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/serve/serve.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/serve/modelswitch_test.go`、`internal/serve/serve_test.go`。
- [ ] 运行相关包测试：`go test ./internal/serve`。
- [ ] 运行相关包 race：`go test -race ./internal/serve`。
- [ ] 运行相关包 vet：`go vet ./internal/serve`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/serve/serve.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/serve/serve.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-356 · 圈复杂度 · `New`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-356 |
| Baseline key | `complexity\|internal/skill/skill.go\|New` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/skill/skill.go` |
| 代码行 | 107 |
| 函数/方法/包 | `New` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`New` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 107-154 行；可见控制流热点：L109 `if home == "" {`；L115 `if suiyuanHome == "" {`；L123 `if root != "" {`；L124 `if abs, err := filepath.Abs(root); err == nil {`；L130 `if wd, err := os.Getwd(); err == nil {`；L140 `if stderr == nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（resolveCustomPaths、normalizeMaxDepth）、状态与控制（disabledNameSet）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function New（internal/skill/skill.go:107-154）
- **可信生产调用方（7）：** `Build`（`internal/boot/boot.go:152`，调用点 L285/L295，confidence=0.90，import，2 条边）；`skillRootsViewFrom`（`desktop/app.go:4932`，调用点 L4941，confidence=0.90，import）；`skillStoreForWorkspace`（`desktop/memory_suggestions.go:483`，调用点 L495，confidence=0.90，import）；`newReviewSkillStore`（`internal/cli/review.go:106`，调用点 L113，confidence=0.90，import）；`chatTUI::skillStore`（`internal/cli/skill_hooks.go:225`，调用点 L235，confidence=0.90，import）；`installSourceTool::applySkillRoot`（`internal/installsource/apply.go:51`，调用点 L59，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `resolveCustomPaths`（`internal/skill/skill.go:741`，调用点 L134/L136，confidence=0.90，exact-match，2 条边）；`dedupePaths`（`internal/skill/skill.go:763`，调用点 L134/L136，confidence=0.90，exact-match，2 条边）；`CanonicalSkillPath`（`internal/config/edit.go:670`，调用点 L137，confidence=0.90，import）；`SuiYuanHomeDir`（`internal/config/paths.go:395`，调用点 L119，confidence=0.90，import）；`disabledNameSet`（`internal/skill/skill.go:234`，调用点 L149，confidence=0.90，exact-match）；`normalizeMaxDepth`（`internal/skill/skill.go:248`，调用点 L150，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `skillRootsView`（`desktop/app.go:4925`，调用点 L4929，confidence=0.90，exact-match，待源码复核）；`buildController`（`internal/automation/runner.go:65`，调用点 L66，confidence=0.90，import，待源码复核）；`Build`（`internal/boot/boot.go:152`，调用点 L285，confidence=0.90，import，待源码复核）；`installSourceTool::apply`（`internal/installsource/apply.go:18`，调用点 L23，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 10 个文件；`internal/skill`、`internal/boot`、`desktop`、`internal/cli`、`internal/installsource`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/skill/skill_extra_test.go`、`internal/skill/skill_test.go`、`internal/skill/tools_test.go`、`internal/control/input_test.go`、`internal/installsource/install_source_test.go`
- **同目录测试候选：** `internal/skill/builtins_test.go`
- **需同步检查：** `internal/skill/skill.go`、`internal/boot/boot.go`、`desktop/app.go`、`desktop/memory_suggestions.go`、`internal/cli/review.go`、`internal/config/edit.go`、`internal/config/paths.go`、`internal/skill/skill_extra_test.go`、`internal/skill/skill_test.go`、`internal/skill/tools_test.go`、`internal/control/input_test.go`

#### 待修改项

- [ ] 逐段核对 L109、L115、L124、L130、L140 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `New` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/skill/skill.go|New` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/skill/skill.go:107-154` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `New` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/skill/skill_extra_test.go`、`internal/skill/skill_test.go`、`internal/skill/tools_test.go`、`internal/control/input_test.go`、`internal/installsource/install_source_test.go`。
- [ ] 运行相关包测试：`go test ./internal/skill`。
- [ ] 运行相关包 vet：`go vet ./internal/skill`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/skill/skill.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/skill/skill.go|New` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-352 · 圈复杂度 · `safeRedirectTarget`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-352 |
| Baseline key | `complexity\|internal/serve/auth.go\|safeRedirectTarget` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/serve/auth.go` |
| 代码行 | 471 |
| 函数/方法/包 | `safeRedirectTarget` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`safeRedirectTarget` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 471-495 行；可见控制流热点：L474 `if i := strings.IndexByte(raw, '#'); i >= 0 {`；L477 `if raw == "" {`；L480 `if raw != "/" && (len(raw) <= 1 || raw[0] != '/' || raw[1] == '/' || raw[1] == '\\') {`；L484 `if err != nil || u == nil || u.IsAbs() || u.Hostname() != "" {`；L488 `if path == "" {`；L491 `if path != "/" && (len(path) <= 1 || path[0] != '/' || path[1] == '/' || path[1] == '\\') {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function safeRedirectTarget（internal/serve/auth.go:471-495）
- **可信生产调用方（3）：** `authGate::checkSession`（`internal/serve/auth.go:335`，调用点 L347，confidence=0.90，exact-match）；`authGate::loginSubmit`（`internal/serve/auth.go:390`，调用点 L449，confidence=0.90，exact-match）；`redirectToSafeTarget`（`internal/serve/auth.go:497`，调用点 L498，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/serve`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/serve/auth_test.go`
- **同目录测试候选：** `internal/serve/actor_test.go`、`internal/serve/automation_test.go`、`internal/serve/broadcaster_test.go`、`internal/serve/csrf_test.go`、`internal/serve/effort_test.go`、`internal/serve/modelswitch_test.go`、`internal/serve/serve_test.go`、`internal/serve/titlecache_test.go`
- **需同步检查：** `internal/serve/auth.go`、`internal/serve/auth_test.go`、`internal/serve/actor_test.go`、`internal/serve/automation_test.go`、`internal/serve/broadcaster_test.go`

#### 待修改项

- [ ] 逐段核对 L474、L477、L484、L488、L491 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `safeRedirectTarget` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/serve/auth.go|safeRedirectTarget` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/serve/auth.go:471-495` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `safeRedirectTarget` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/serve/auth_test.go`。
- [ ] 运行相关包测试：`go test ./internal/serve`。
- [ ] 运行相关包 vet：`go vet ./internal/serve`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/serve/auth.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/serve/auth.go|safeRedirectTarget` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-355 · 圈复杂度 · `*Server.deleteSession`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-355 |
| Baseline key | `complexity\|internal/serve/serve.go\|*Server.deleteSession` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 35） |
| 文件 | `internal/serve/serve.go` |
| 代码行 | 1052 |
| 函数/方法/包 | `*Server.deleteSession` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Server.deleteSession` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 1052-1110 行；可见控制流热点：L1056 `if err := decodeJSONRequest(w, r, &req); err != nil {`；L1065 `if name == "." || name == ".." || strings.ContainsAny(name, \`/\\`) {`；L1076 `if err != nil {`；L1086 `if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator…`；L1095 `if result := finishSessionDestroy(destroy); result.HasTimedOut() {`；L1105 `if err := removeSessionFiles(absDir, abs); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（decodeJSONRequest）、持久化/I-O（writeJSONRequestError、removeSessionFiles）、网络/协议（decodeJSONRequest、writeJSONRequestError）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Server::deleteSession（internal/serve/serve.go:1052-1110）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `delayedSessionDelete`（`internal/serve/serve.go:1126`，调用点 L1097/L1101，confidence=0.90，exact-match，2 条边）；`MarkCleanupPending`（`internal/agent/save.go:227`，调用点 L1096，confidence=0.90，import）；`decodeJSONRequest`（`internal/serve/request_json.go:11`，调用点 L1056，confidence=0.90，exact-match）；`writeJSONRequestError`（`internal/serve/request_json.go:15`，调用点 L1057，confidence=0.90，exact-match）；`finishSessionDestroy`（`internal/serve/serve.go:1112`，调用点 L1095，confidence=0.90，exact-match）；`removeSessionFiles`（`internal/serve/serve.go:1138`，调用点 L1105，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Server::ctl`（`internal/serve/serve.go:83`，调用点 L1069/L1090/L1094，confidence=0.70，instance-method，3 条边，待源码复核）；`TeardownResult::HasTimedOut`（`internal/jobs/jobs.go:77`，调用点 L1095，confidence=0.80，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 22 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/serve`、`internal/agent`、`internal/jobs`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/serve/actor_test.go`、`internal/serve/auth_test.go`、`internal/serve/automation_test.go`、`internal/serve/broadcaster_test.go`、`internal/serve/csrf_test.go`、`internal/serve/effort_test.go`、`internal/serve/modelswitch_test.go`、`internal/serve/serve_test.go`、`internal/serve/titlecache_test.go`
- **需同步检查：** `internal/serve/serve.go`、`internal/agent/save.go`、`internal/serve/request_json.go`、`internal/serve/actor_test.go`、`internal/serve/auth_test.go`、`internal/serve/automation_test.go`、`internal/serve/broadcaster_test.go`

#### 待修改项

- [ ] 先冻结 `*Server.deleteSession` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1056、L1070、L1081、L1095、L1105 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Server.deleteSession` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/serve/serve.go|*Server.deleteSession` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/serve/serve.go:1052-1110` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Server.deleteSession` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/serve/actor_test.go`、`internal/serve/auth_test.go`、`internal/serve/automation_test.go`、`internal/serve/broadcaster_test.go`、`internal/serve/csrf_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/serve`。
- [ ] 运行相关包 race：`go test -race ./internal/serve`。
- [ ] 运行相关包 vet：`go vet ./internal/serve`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/serve/serve.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/serve/serve.go|*Server.deleteSession` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-358 · 圈复杂度 · `*Store.readEntry`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-358 |
| Baseline key | `complexity\|internal/skill/skill.go\|*Store.readEntry` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/skill/skill.go` |
| 代码行 | 409 |
| 函数/方法/包 | `*Store.readEntry` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Store.readEntry` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 409-442 行；可见控制流热点：L415 `if !isDir && !isFile && shouldStatEntryTarget(e.Type()) {`；L417 `if err != nil {`；L424 `if isDir {`；L429 `if _, err := os.Stat(file); err != nil {`；L434 `if isFile && strings.EqualFold(filepath.Ext(name), ".md") {`；L436 `if !IsValidName(stem) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Store::parse、Store::parseFlat）、状态与控制（shouldStatEntryTarget）、持久化/I-O（Store::parse、Store::parseFlat）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::readEntry（internal/skill/skill.go:409-442）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Store::scanDir`（`internal/skill/skill.go:345`，调用点 L360，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `IsValidName`（`internal/skill/skill.go:71`，调用点 L425/L436，confidence=0.90，exact-match，2 条边）；`shouldStatEntryTarget`（`internal/skill/skill.go:389`，调用点 L415，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Store::parse`（`internal/skill/skill.go:447`，调用点 L432，confidence=0.70，instance-method，待源码复核）；`Store::parseFlat`（`internal/skill/skill.go:454`，调用点 L439，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/skill`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/skill/skill_test.go`
- **同目录测试候选：** `internal/skill/builtins_test.go`、`internal/skill/skill_extra_test.go`、`internal/skill/tools_test.go`
- **需同步检查：** `internal/skill/skill.go`、`internal/skill/skill_test.go`、`internal/skill/builtins_test.go`、`internal/skill/skill_extra_test.go`、`internal/skill/tools_test.go`

#### 待修改项

- [ ] 逐段核对 L415、L424、L425、L434、L436 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Store.readEntry` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/skill/skill.go|*Store.readEntry` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/skill/skill.go:409-442` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Store.readEntry` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/skill/skill_test.go`。
- [ ] 运行相关包测试：`go test ./internal/skill`。
- [ ] 运行相关包 vet：`go vet ./internal/skill`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/skill/skill.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/skill/skill.go|*Store.readEntry` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-360 · 圈复杂度 · `*installSkillTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-360 |
| Baseline key | `complexity\|internal/skill/tools.go\|*installSkillTool.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/skill/tools.go` |
| 代码行 | 445 |
| 函数/方法/包 | `*installSkillTool.Execute` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*installSkillTool.Execute` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 445-508 行；可见控制流热点：L456 `if err := json.Unmarshal(args, &p); err != nil {`；L464 `if desc == "" {`；L472 `switch strings.TrimSpace(p.Scope) {`；L478 `if t.store.HasProjectScope() {`；L487 `if strings.TrimSpace(p.RunAs) == "subagent" {`；L496 `if t.onInstalled != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（Store::HasProjectScope、Store::CreateWithContent、renderSkillFile）、渲染/输出（renderSkillFile）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method installSkillTool::Execute（internal/skill/tools.go:445-508）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `Store::HasProjectScope`（`internal/skill/skill.go:157`，调用点 L478/L482，confidence=0.90，exact-match，2 条边）；`Store::CreateWithContent`（`internal/skill/skill.go:562`，调用点 L492，confidence=0.90，exact-match）；`renderSkillFile`（`internal/skill/tools.go:512`，调用点 L491，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `collapseSpaces`（`internal/skill/tools.go:584`，调用点 L460，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/skill`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/skill/builtins_test.go`、`internal/skill/skill_extra_test.go`、`internal/skill/skill_test.go`、`internal/skill/tools_test.go`
- **需同步检查：** `internal/skill/tools.go`、`internal/skill/skill.go`、`internal/skill/builtins_test.go`、`internal/skill/skill_extra_test.go`、`internal/skill/skill_test.go`、`internal/skill/tools_test.go`

#### 待修改项

- [ ] 逐段核对 L456、L467、L475、L482、L496 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*installSkillTool.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/skill/tools.go|*installSkillTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/skill/tools.go:445-508` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*installSkillTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/skill/builtins_test.go`、`internal/skill/skill_extra_test.go`、`internal/skill/skill_test.go`、`internal/skill/tools_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/skill`。
- [ ] 运行相关包 vet：`go vet ./internal/skill`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/skill/tools.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/skill/tools.go|*installSkillTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-357 · 圈复杂度 · `*Store.List`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-357 |
| Baseline key | `complexity\|internal/skill/skill.go\|*Store.List` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/skill/skill.go` |
| 代码行 | 289 |
| 函数/方法/包 | `*Store.List` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Store.List` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 289-320 行；可见控制流热点：L291 `for _, r := range s.roots() {`；L295 `for _, sk := range s.discoverRoot(r) {`；L299 `if _, dup := byName[sk.Name]; !dup {`；L304 `if !s.disableBuiltins {`；L306 `if s.disabledName(sk.Name) {`；L315 `for _, sk := range byName {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（Store::disabledName、Store::roots、Store::discoverRoot）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::List（internal/skill/skill.go:289-320）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `builtinSkills`（`internal/skill/builtins.go:283`，调用点 L305，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `Store::disabledName`（`internal/skill/skill.go:244`，调用点 L296/L306，confidence=0.70，instance-method，2 条边，待源码复核）；`Store::roots`（`internal/skill/skill.go:186`，调用点 L291，confidence=0.70，instance-method，待源码复核）；`Store::discoverRoot`（`internal/skill/skill.go:339`，调用点 L295，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/skill`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/skill/builtins_test.go`、`internal/skill/skill_extra_test.go`、`internal/skill/skill_test.go`、`internal/skill/tools_test.go`
- **需同步检查：** `internal/skill/skill.go`、`internal/skill/builtins.go`、`internal/skill/builtins_test.go`、`internal/skill/skill_extra_test.go`、`internal/skill/skill_test.go`、`internal/skill/tools_test.go`

#### 待修改项

- [ ] 逐段核对 L291、L295、L304、L306、L315 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Store.List` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/skill/skill.go|*Store.List` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/skill/skill.go:289-320` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Store.List` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/skill/builtins_test.go`、`internal/skill/skill_extra_test.go`、`internal/skill/skill_test.go`、`internal/skill/tools_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/skill`。
- [ ] 运行相关包 vet：`go vet ./internal/skill`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/skill/skill.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/skill/skill.go|*Store.List` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-359 · 圈复杂度 · `*runSkillTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-359 |
| Baseline key | `complexity\|internal/skill/tools.go\|*runSkillTool.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/skill/tools.go` |
| 代码行 | 76 |
| 函数/方法/包 | `*runSkillTool.Execute` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*runSkillTool.Execute` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 76-113 行；可见控制流热点：L83 `if err := json.Unmarshal(args, &p); err != nil {`；L87 `if name == "" {`；L96 `if opts.ContinueFrom != "" && opts.ForkFrom != "" {`；L100 `if sk.RunAs == RunSubagent {`；L104 `if rawArgs == "" {`；L109 `if opts.ContinueFrom != "" || opts.ForkFrom != "" {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（Store::Read）、渲染/输出（renderInline）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method runSkillTool::Execute（internal/skill/tools.go:76-113）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `renderInline`（`internal/skill/tools.go:559`，调用点 L112，confidence=0.90，exact-match）；`cleanSkillName`（`internal/skill/tools.go:568`，调用点 L86，confidence=0.90，exact-match）；`availableNames`（`internal/skill/tools.go:589`，调用点 L92，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Store::Read`（`internal/skill/skill.go:324`，调用点 L90，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/skill`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/skill/builtins_test.go`、`internal/skill/skill_extra_test.go`、`internal/skill/skill_test.go`、`internal/skill/tools_test.go`
- **需同步检查：** `internal/skill/tools.go`、`internal/skill/skill.go`、`internal/skill/builtins_test.go`、`internal/skill/skill_extra_test.go`、`internal/skill/skill_test.go`、`internal/skill/tools_test.go`

#### 待修改项

- [ ] 逐段核对 L83、L91、L100、L101、L109 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*runSkillTool.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/skill/tools.go|*runSkillTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/skill/tools.go:76-113` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*runSkillTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/skill/builtins_test.go`、`internal/skill/skill_extra_test.go`、`internal/skill/skill_test.go`、`internal/skill/tools_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/skill`。
- [ ] 运行相关包 vet：`go vet ./internal/skill`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/skill/tools.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/skill/tools.go|*runSkillTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-354 · 圈复杂度 · `*Server.generateTitle`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-354 |
| Baseline key | `complexity\|internal/serve/serve.go\|*Server.generateTitle` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 32） |
| 文件 | `internal/serve/serve.go` |
| 代码行 | 963 |
| 函数/方法/包 | `*Server.generateTitle` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Server.generateTitle` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 963-1001 行；可见控制流热点：L964 `if nilutil.IsNil(s.titleProv) || strings.TrimSpace(firstMsg) == "" {`；L978 `if err != nil {`；L984 `switch chunk.Type {`；L985 `case provider.ChunkText:`；L989 `case provider.ChunkError:`；L997 `if len(title) >= 2 && ((title[0] == '"' && title[len(title)-1] == '"') || (title[0] == '\'' && titl…`。CodeGraph 记录的下游职责候选主要涉及 渲染/输出（Broadcaster::Emit）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Server::generateTitle（internal/serve/serve.go:963-1001）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Server::sessionTitle`（`internal/serve/serve.go:1168`，调用点 L1172，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `IsNil`（`internal/nilutil/nil.go:6`，调用点 L964，confidence=0.90，import）
- **待源码复核被调候选（1）：** `Broadcaster::Emit`（`internal/serve/broadcaster.go:29`，调用点 L994，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `Server::sessions`（`internal/serve/serve.go:1005`，调用点 L1037，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 11 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/serve`、`internal/nilutil`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/serve/actor_test.go`、`internal/serve/auth_test.go`、`internal/serve/automation_test.go`、`internal/serve/broadcaster_test.go`、`internal/serve/csrf_test.go`、`internal/serve/effort_test.go`、`internal/serve/modelswitch_test.go`、`internal/serve/serve_test.go`、`internal/serve/titlecache_test.go`
- **需同步检查：** `internal/serve/serve.go`、`internal/nilutil/nil.go`、`internal/serve/broadcaster.go`、`internal/serve/actor_test.go`、`internal/serve/auth_test.go`、`internal/serve/automation_test.go`、`internal/serve/broadcaster_test.go`

#### 待修改项

- [ ] 先冻结 `*Server.generateTitle` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L964、L978、L985、L989、L997 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Server.generateTitle` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/serve/serve.go|*Server.generateTitle` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/serve/serve.go:963-1001` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Server.generateTitle` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/serve/actor_test.go`、`internal/serve/auth_test.go`、`internal/serve/automation_test.go`、`internal/serve/broadcaster_test.go`、`internal/serve/csrf_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/serve`。
- [ ] 运行相关包 vet：`go vet ./internal/serve`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/serve/serve.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/serve/serve.go|*Server.generateTitle` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-353 · 圈复杂度 · `*Server.resume`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-353 |
| Baseline key | `complexity\|internal/serve/serve.go\|*Server.resume` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 30） |
| 文件 | `internal/serve/serve.go` |
| 代码行 | 798 |
| 函数/方法/包 | `*Server.resume` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Server.resume` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 798-845 行；可见控制流热点：L802 `if err := decodeJSONRequest(w, r, &body); err != nil || body.Path == "" {`；L812 `if err != nil {`；L817 `if err != nil {`；L826 `if err != nil {`；L830 `if agent.IsCleanupPending(realPath) {`；L839 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（LoadSession、decodeJSONRequest、resolveServeSessionPath）、持久化/I-O（writeJSONRequestError）、网络/协议（decodeJSONRequest、writeJSONRequestError）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Server::resume（internal/serve/serve.go:798-845）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `LoadSession`（`internal/agent/save.go:62`，调用点 L838，confidence=0.90，import）；`IsCleanupPending`（`internal/agent/save.go:256`，调用点 L830，confidence=0.90，import）；`decodeJSONRequest`（`internal/serve/request_json.go:11`，调用点 L802，confidence=0.90，exact-match）；`writeJSONRequestError`（`internal/serve/request_json.go:15`，调用点 L803，confidence=0.90，exact-match）；`resolveServeSessionPath`（`internal/serve/serve.go:847`，调用点 L821，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Server::ctl`（`internal/serve/serve.go:83`，调用点 L806/L835/L843，confidence=0.70，instance-method，3 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 11 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/serve`、`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/serve/actor_test.go`、`internal/serve/auth_test.go`、`internal/serve/automation_test.go`、`internal/serve/broadcaster_test.go`、`internal/serve/csrf_test.go`、`internal/serve/effort_test.go`、`internal/serve/modelswitch_test.go`、`internal/serve/serve_test.go`、`internal/serve/titlecache_test.go`
- **需同步检查：** `internal/serve/serve.go`、`internal/agent/save.go`、`internal/serve/request_json.go`、`internal/serve/actor_test.go`、`internal/serve/auth_test.go`、`internal/serve/automation_test.go`、`internal/serve/broadcaster_test.go`

#### 待修改项

- [ ] 先冻结 `*Server.resume` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L802、L812、L822、L830、L839 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Server.resume` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/serve/serve.go|*Server.resume` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/serve/serve.go:798-845` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Server.resume` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/serve/actor_test.go`、`internal/serve/auth_test.go`、`internal/serve/automation_test.go`、`internal/serve/broadcaster_test.go`、`internal/serve/csrf_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/serve`。
- [ ] 运行相关包 vet：`go vet ./internal/serve`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/serve/serve.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/serve/serve.go|*Server.resume` 下降或消失且无新增/恶化。
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
