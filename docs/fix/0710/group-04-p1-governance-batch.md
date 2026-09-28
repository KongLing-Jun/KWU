# G04 · 配置渲染与差异输出

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G04 |
| 问题数量 | 10 |
| 整体优先级 | P1 |
| 涉及模块 | `internal/config` |
| 涉及文件 | `internal/config/render.go`<br>`internal/config/effort.go`<br>`internal/config/edit.go` |
| 主要治理目标 | 围绕 `internal/config/render.go` 等治理对象降低复杂度/体量，统一处理 2 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | config 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`；单文件最多集中 6 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 4 组；先于 G43 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **64/100**，对象 `RenderTOMLForScope` 的分项为严重度 22.8、集中度 11.4、影响面 22.2、可执行性 8.0；最高超限 9.80x，文件内 6 条问题，CodeGraph 记录 2 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数，必须合并治理。

### 治理边界与组间关系

本组涉及 3 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G43，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-195` | `complexity\|internal/config/render.go\|RenderTOMLForScope` | 圈复杂度 | `internal/config/render.go` | `RenderTOMLForScope` | 32 | 98/10 | +88（9.80x） | P1 | 2 可信 + 0 待核 caller / 18 可信 + 25 待核 callee / 288 unresolved | 待修改 |
| `FN-030` | `func_lines\|internal/config/render.go\|RenderTOMLForScope` | 函数有效行数 | `internal/config/render.go` | `RenderTOMLForScope` | 32 | 516/80 | +436（6.45x） | P1 | 2 可信 + 0 待核 caller / 18 可信 + 25 待核 callee / 288 unresolved | 待修改 |
| `CX-196` | `complexity\|internal/config/render.go\|RenderTOMLProjectDelta` | 圈复杂度 | `internal/config/render.go` | `RenderTOMLProjectDelta` | 575 | 124/10 | +114（12.40x） | P1 | 0 可信 + 0 待核 caller / 9 可信 + 11 待核 callee / 149 unresolved | 待修改 |
| `FN-031` | `func_lines\|internal/config/render.go\|RenderTOMLProjectDelta` | 函数有效行数 | `internal/config/render.go` | `RenderTOMLProjectDelta` | 575 | 338/80 | +258（4.22x） | P1 | 0 可信 + 0 待核 caller / 9 可信 + 11 待核 callee / 149 unresolved | 待修改 |
| `CX-181` | `complexity\|internal/config/effort.go\|NormalizeEffort` | 圈复杂度 | `internal/config/effort.go` | `NormalizeEffort` | 86 | 20/10 | +10（2.00x） | P2 | 10 可信 + 0 待核 caller / 6 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-180` | `complexity\|internal/config/effort.go\|EffortCapabilityForEntry` | 圈复杂度 | `internal/config/effort.go` | `EffortCapabilityForEntry` | 39 | 13/10 | +3（1.30x） | P2 | 9 可信 + 0 待核 caller / 9 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `FL-009` | `file_lines\|internal/config/edit.go\|` | 文件有效行数 | `internal/config/edit.go` | `internal/config/edit.go` | 1 | 1018/800 | +218（1.27x） | P2 | 6 可信 + 7 待核 caller / 12 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-179` | `complexity\|internal/config/edit.go\|replaceTOMLSection` | 圈复杂度 | `internal/config/edit.go` | `replaceTOMLSection` | 1020 | 18/10 | +8（1.80x） | P3 | 2 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `FL-011` | `file_lines\|internal/config/render.go\|` | 文件有效行数 | `internal/config/render.go` | `internal/config/render.go` | 1 | 1151/800 | +351（1.44x） | P3 | 1 可信 + 0 待核 caller / 3 可信 + 26 待核 callee / 524 unresolved | 待修改 |
| `CX-197` | `complexity\|internal/config/render.go\|isBareTOMLKey` | 圈复杂度 | `internal/config/render.go` | `isBareTOMLKey` | 1118 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-195 · 圈复杂度 · `RenderTOMLForScope`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-195 |
| Baseline key | `complexity\|internal/config/render.go\|RenderTOMLForScope` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 64） |
| 文件 | `internal/config/render.go` |
| 代码行 | 32 |
| 函数/方法/包 | `RenderTOMLForScope` |
| Baseline / 当前复测 / 阈值 | 98 / 98 / 10 |
| 超出 | +88，9.80x |
| 同一治理对象关联问题 | `FN-030` |

#### 明确的问题描述

`RenderTOMLForScope` 的圈复杂度当前为 **98**，超过限制 **10**（+88，9.80x）。定义范围为 32-569 行；可见控制流热点：L33 `if c == nil {`；L155 `if c.Network.Proxy.Server != "" {`；L254 `if c.Agent.PlannerModel != "" {`；L331 `if p.ReasoningProtocol != "" {`；L453 `if c.Bot.ToolApprovalMode != "" {`；L562 `if pl.AutoStart != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeCredentialsStore、userConfigDisplayPath、configVersion、projectScopedConfigForRender）、状态与控制（Config::DesktopCheckUpdates、Config::ColdResumePruneEnabled）、持久化/I-O（normalizeCredentialsStore、Config::DesktopCloseBehavior）、渲染/输出（renderStringArray、renderStringMap、formatFloat、renderRuleList）、网络/协议（shouldRenderProviders）、任务/并发（Config::BackgroundJobStalledWarningSeconds）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function RenderTOMLForScope（internal/config/render.go:32-569）
- **可信生产调用方（2）：** `Config::WriteFile`（`internal/config/config.go:1293`，调用点 L1298，confidence=0.90，exact-match）；`RenderTOML`（`internal/config/render.go:24`，调用点 L25，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（18）：** `renderStringArray`（`internal/config/render.go:1132`，调用点 L104/L110/L239/L250，confidence=0.90，exact-match，19 条边）；`renderStringMap`（`internal/config/render.go:1147`，调用点 L265/L275/L553/L556，confidence=0.90，exact-match，4 条边）；`formatFloat`（`internal/config/render.go:1244`，调用点 L213/L235/L236/L237，confidence=0.90，exact-match，4 条边）；`renderRuleList`（`internal/config/render.go:1226`，调用点 L410/L411/L412，confidence=0.90，exact-match，3 条边）；`normalizeCredentialsStore`（`internal/config/credentials.go:184`，调用点 L60，confidence=0.90，exact-match）；`userConfigDisplayPath`（`internal/config/paths.go:363`，调用点 L48，confidence=0.90，exact-match）；`renderPricingInline`（`internal/config/render.go:948`，调用点 L311，confidence=0.90，exact-match）；`renderPricingMap`（`internal/config/render.go:956`，调用点 L314，confidence=0.90，exact-match）
- **待源码复核被调候选（25）：** `Config::DesktopCloseBehavior`（`internal/config/config.go:234`，调用点 L78/L102，confidence=0.70，instance-method，2 条边，待源码复核）；`Default`（`internal/config/config.go:1230`，调用点 L34/L44，confidence=0.70，exact-match，2 条边，待源码复核）；`Config::UITheme`（`internal/config/config.go:127`，调用点 L66，confidence=0.70，instance-method，待源码复核）；`Config::UIThemeStyle`（`internal/config/config.go:140`，调用点 L67，confidence=0.70，instance-method，待源码复核）；`Config::UIShortcutLayout`（`internal/config/config.go:147`，调用点 L72，confidence=0.70，instance-method，待源码复核）；`Config::DesktopLanguage`（`internal/config/config.go:190`，调用点 L90，confidence=0.70，instance-method，待源码复核）；`Config::DesktopTheme`（`internal/config/config.go:203`，调用点 L96，confidence=0.70，instance-method，待源码复核）；`Config::DesktopThemeStyle`（`internal/config/config.go:218`，调用点 L97，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Config::WriteFile`（`internal/config/config.go:1293`，调用点 L1298，confidence=0.90，exact-match，待源码复核）；`RenderTOML`（`internal/config/render.go:24`，调用点 L25，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 288 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/config/render_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/render.go`、`internal/config/config.go`、`internal/config/render_test.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`

#### 待修改项

- [ ] 先冻结 `RenderTOMLForScope` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L33、L186、L300、L420、L562 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `RenderTOMLForScope` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-195、FN-030 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/render.go|RenderTOMLForScope` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/render.go:32-569` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `RenderTOMLForScope` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-195`、`FN-030` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/config/render_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 race：`go test -race ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/render.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/render.go|RenderTOMLForScope` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-030 · 函数有效行数 · `RenderTOMLForScope`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-030 |
| Baseline key | `func_lines\|internal/config/render.go\|RenderTOMLForScope` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 64） |
| 文件 | `internal/config/render.go` |
| 代码行 | 32 |
| 函数/方法/包 | `RenderTOMLForScope` |
| Baseline / 当前复测 / 阈值 | 516 / 516 / 80 |
| 超出 | +436，6.45x |
| 同一治理对象关联问题 | `CX-195` |

#### 明确的问题描述

`RenderTOMLForScope` 的有效代码行 **516**，超过限制 **80**（+436）。CodeGraph 定义物理范围为 32-569 行，并记录 43 个直接下游候选；候选职责分布为 配置/解析（normalizeCredentialsStore、userConfigDisplayPath、configVersion、projectScopedConfigForRender）、状态与控制（Config::DesktopCheckUpdates、Config::ColdResumePruneEnabled）、持久化/I-O（normalizeCredentialsStore、Config::DesktopCloseBehavior）、渲染/输出（renderStringArray、renderStringMap、formatFloat、renderRuleList）、网络/协议（shouldRenderProviders）、任务/并发（Config::BackgroundJobStalledWarningSeconds）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function RenderTOMLForScope（internal/config/render.go:32-569）
- **可信生产调用方（2）：** `Config::WriteFile`（`internal/config/config.go:1293`，调用点 L1298，confidence=0.90，exact-match）；`RenderTOML`（`internal/config/render.go:24`，调用点 L25，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（18）：** `renderStringArray`（`internal/config/render.go:1132`，调用点 L104/L110/L239/L250，confidence=0.90，exact-match，19 条边）；`renderStringMap`（`internal/config/render.go:1147`，调用点 L265/L275/L553/L556，confidence=0.90，exact-match，4 条边）；`formatFloat`（`internal/config/render.go:1244`，调用点 L213/L235/L236/L237，confidence=0.90，exact-match，4 条边）；`renderRuleList`（`internal/config/render.go:1226`，调用点 L410/L411/L412，confidence=0.90，exact-match，3 条边）；`normalizeCredentialsStore`（`internal/config/credentials.go:184`，调用点 L60，confidence=0.90，exact-match）；`userConfigDisplayPath`（`internal/config/paths.go:363`，调用点 L48，confidence=0.90，exact-match）；`renderPricingInline`（`internal/config/render.go:948`，调用点 L311，confidence=0.90，exact-match）；`renderPricingMap`（`internal/config/render.go:956`，调用点 L314，confidence=0.90，exact-match）
- **待源码复核被调候选（25）：** `Config::DesktopCloseBehavior`（`internal/config/config.go:234`，调用点 L78/L102，confidence=0.70，instance-method，2 条边，待源码复核）；`Default`（`internal/config/config.go:1230`，调用点 L34/L44，confidence=0.70，exact-match，2 条边，待源码复核）；`Config::UITheme`（`internal/config/config.go:127`，调用点 L66，confidence=0.70，instance-method，待源码复核）；`Config::UIThemeStyle`（`internal/config/config.go:140`，调用点 L67，confidence=0.70，instance-method，待源码复核）；`Config::UIShortcutLayout`（`internal/config/config.go:147`，调用点 L72，confidence=0.70，instance-method，待源码复核）；`Config::DesktopLanguage`（`internal/config/config.go:190`，调用点 L90，confidence=0.70，instance-method，待源码复核）；`Config::DesktopTheme`（`internal/config/config.go:203`，调用点 L96，confidence=0.70，instance-method，待源码复核）；`Config::DesktopThemeStyle`（`internal/config/config.go:218`，调用点 L97，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Config::WriteFile`（`internal/config/config.go:1293`，调用点 L1298，confidence=0.90，exact-match，待源码复核）；`RenderTOML`（`internal/config/render.go:24`，调用点 L25，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 288 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/config/render_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/render.go`、`internal/config/config.go`、`internal/config/render_test.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`

#### 待修改项

- [ ] 先冻结 `RenderTOMLForScope` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `RenderTOMLForScope` 的 516 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-195、FN-030 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/config/render.go|RenderTOMLForScope` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/render.go:32-569` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `RenderTOMLForScope` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-195`、`FN-030` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/config/render_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 race：`go test -race ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/render.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/config/render.go|RenderTOMLForScope` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-196 · 圈复杂度 · `RenderTOMLProjectDelta`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-196 |
| Baseline key | `complexity\|internal/config/render.go\|RenderTOMLProjectDelta` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 63） |
| 文件 | `internal/config/render.go` |
| 代码行 | 575 |
| 函数/方法/包 | `RenderTOMLProjectDelta` |
| Baseline / 当前复测 / 阈值 | 124 / 124 / 10 |
| 超出 | +114，12.40x |
| 同一治理对象关联问题 | `FN-031` |

#### 明确的问题描述

`RenderTOMLProjectDelta` 的圈复杂度当前为 **124**，超过限制 **10**（+114，12.40x）。定义范围为 575-946 行；可见控制流热点：L576 `if c == nil {`；L644 `if c.Network.Proxy.Password != "" {`；L727 `if c.Agent.SubagentEffort != "" && c.Agent.SubagentEffort != d.Agent.SubagentEffort {`；L796 `if len(p.SupportedEfforts) > 0 {`；L871 `if mode != "ask" {`；L939 `if pl.AutoStart != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（configVersion、projectScopedConfigForRender、renderLSPConfig、Config::UIThemeStyle）、状态与控制（Config::ColdResumePruneEnabled）、持久化/I-O（renderProfileDelta、Config::DesktopCloseBehavior）、渲染/输出（renderStringArray、renderStringMap、formatFloat、renderPricingInline）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function RenderTOMLProjectDelta（internal/config/render.go:575-946）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（9）：** `renderStringArray`（`internal/config/render.go:1132`，调用点 L700/L712/L755/L788，confidence=0.90，exact-match，15 条边）；`renderStringMap`（`internal/config/render.go:1147`，调用点 L724/L732/L930/L933，confidence=0.90，exact-match，4 条边）；`formatFloat`（`internal/config/render.go:1244`，调用点 L674/L688/L692/L696，confidence=0.90，exact-match，4 条边）；`renderPricingInline`（`internal/config/render.go:948`，调用点 L773，confidence=0.90，exact-match）；`renderPricingMap`（`internal/config/render.go:956`，调用点 L776，confidence=0.90，exact-match）；`configVersion`（`internal/config/render.go:982`，调用点 L583，confidence=0.90，exact-match）；`renderProfileDelta`（`internal/config/render.go:989`，调用点 L593，confidence=0.90，exact-match）；`projectScopedConfigForRender`（`internal/config/render.go:1039`，调用点 L747，confidence=0.90，exact-match）
- **待源码复核被调候选（11）：** `Config::UIThemeStyle`（`internal/config/config.go:140`，调用点 L601，confidence=0.70，instance-method，2 条边，待源码复核）；`Config::BashMode`（`internal/config/config.go:805`，调用点 L895/L896，confidence=0.70，instance-method，2 条边，待源码复核）；`Config::UITheme`（`internal/config/config.go:127`，调用点 L599，confidence=0.70，instance-method，待源码复核）；`Config::UIShortcutLayout`（`internal/config/config.go:147`，调用点 L604，confidence=0.70，instance-method，待源码复核）；`Config::DesktopCloseBehavior`（`internal/config/config.go:234`，调用点 L608，confidence=0.70，instance-method，待源码复核）；`Config::ColdResumePruneEnabled`（`internal/config/config.go:363`，调用点 L708，confidence=0.70，instance-method，待源码复核）；`Config::ReasoningLanguage`（`internal/config/config.go:396`，调用点 L678，confidence=0.70，instance-method，待源码复核）；`Config::NetworkProxyMode`（`internal/config/config.go:656`，调用点 L620，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 149 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/config/render_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/render.go`、`internal/config/render_test.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`

#### 待修改项

- [ ] 先冻结 `RenderTOMLProjectDelta` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L576、L673、L769、L853、L939 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `RenderTOMLProjectDelta` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-196、FN-031 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/render.go|RenderTOMLProjectDelta` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/render.go:575-946` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `RenderTOMLProjectDelta` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-196`、`FN-031` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/config/render_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/render.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/render.go|RenderTOMLProjectDelta` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-031 · 函数有效行数 · `RenderTOMLProjectDelta`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-031 |
| Baseline key | `func_lines\|internal/config/render.go\|RenderTOMLProjectDelta` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 63） |
| 文件 | `internal/config/render.go` |
| 代码行 | 575 |
| 函数/方法/包 | `RenderTOMLProjectDelta` |
| Baseline / 当前复测 / 阈值 | 338 / 338 / 80 |
| 超出 | +258，4.22x |
| 同一治理对象关联问题 | `CX-196` |

#### 明确的问题描述

`RenderTOMLProjectDelta` 的有效代码行 **338**，超过限制 **80**（+258）。CodeGraph 定义物理范围为 575-946 行，并记录 20 个直接下游候选；候选职责分布为 配置/解析（configVersion、projectScopedConfigForRender、renderLSPConfig、Config::UIThemeStyle）、状态与控制（Config::ColdResumePruneEnabled）、持久化/I-O（renderProfileDelta、Config::DesktopCloseBehavior）、渲染/输出（renderStringArray、renderStringMap、formatFloat、renderPricingInline）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function RenderTOMLProjectDelta（internal/config/render.go:575-946）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（9）：** `renderStringArray`（`internal/config/render.go:1132`，调用点 L700/L712/L755/L788，confidence=0.90，exact-match，15 条边）；`renderStringMap`（`internal/config/render.go:1147`，调用点 L724/L732/L930/L933，confidence=0.90，exact-match，4 条边）；`formatFloat`（`internal/config/render.go:1244`，调用点 L674/L688/L692/L696，confidence=0.90，exact-match，4 条边）；`renderPricingInline`（`internal/config/render.go:948`，调用点 L773，confidence=0.90，exact-match）；`renderPricingMap`（`internal/config/render.go:956`，调用点 L776，confidence=0.90，exact-match）；`configVersion`（`internal/config/render.go:982`，调用点 L583，confidence=0.90，exact-match）；`renderProfileDelta`（`internal/config/render.go:989`，调用点 L593，confidence=0.90，exact-match）；`projectScopedConfigForRender`（`internal/config/render.go:1039`，调用点 L747，confidence=0.90，exact-match）
- **待源码复核被调候选（11）：** `Config::UIThemeStyle`（`internal/config/config.go:140`，调用点 L601，confidence=0.70，instance-method，2 条边，待源码复核）；`Config::BashMode`（`internal/config/config.go:805`，调用点 L895/L896，confidence=0.70，instance-method，2 条边，待源码复核）；`Config::UITheme`（`internal/config/config.go:127`，调用点 L599，confidence=0.70，instance-method，待源码复核）；`Config::UIShortcutLayout`（`internal/config/config.go:147`，调用点 L604，confidence=0.70，instance-method，待源码复核）；`Config::DesktopCloseBehavior`（`internal/config/config.go:234`，调用点 L608，confidence=0.70，instance-method，待源码复核）；`Config::ColdResumePruneEnabled`（`internal/config/config.go:363`，调用点 L708，confidence=0.70，instance-method，待源码复核）；`Config::ReasoningLanguage`（`internal/config/config.go:396`，调用点 L678，confidence=0.70，instance-method，待源码复核）；`Config::NetworkProxyMode`（`internal/config/config.go:656`，调用点 L620，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 149 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/config/render_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/render.go`、`internal/config/render_test.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`

#### 待修改项

- [ ] 先冻结 `RenderTOMLProjectDelta` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `RenderTOMLProjectDelta` 的 338 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L582 `Top-level scalar fields`、L746 `[[providers]] — include user-defined providers that aren't built-in`、L847 `[skills]`、L913 `[[plugins]] — always include when set; replaces all existing entries` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-196、FN-031 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/config/render.go|RenderTOMLProjectDelta` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/render.go:575-946` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `RenderTOMLProjectDelta` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-196`、`FN-031` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/config/render_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/render.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/config/render.go|RenderTOMLProjectDelta` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-181 · 圈复杂度 · `NormalizeEffort`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-181 |
| Baseline key | `complexity\|internal/config/effort.go\|NormalizeEffort` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 53） |
| 文件 | `internal/config/effort.go` |
| 代码行 | 86 |
| 函数/方法/包 | `NormalizeEffort` |
| Baseline / 当前复测 / 阈值 | 20 / 20 / 10 |
| 超出 | +10，2.00x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`NormalizeEffort` 的圈复杂度当前为 **20**，超过限制 **10**（+10，2.00x）。定义范围为 86-153 行；可见控制流热点：L88 `if level == "" {`；L104 `switch ReasoningProtocolForEntry(e) {`；L109 `case "low", "medium":`；L124 `switch {`；L134 `case "off":`；L145 `case "low", "medium", "high", "xhigh", "max":`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（effortNotConfigurableError、normalizeEffortLevel、normalizedSupportedEfforts）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function NormalizeEffort（internal/config/effort.go:86-153）
- **可信生产调用方（10）：** `App::SetEffortForTab`（`desktop/app.go:6159`，调用点 L6167/L6184，confidence=0.90，import，2 条边）；`App::SetModelForTab`（`desktop/app.go:6025`，调用点 L6075，confidence=0.90，import）；`App::runEffortCommandForTab`（`desktop/app.go:6746`，调用点 L6766，confidence=0.90，import）；`App::SetSubagentEffort`（`desktop/settings_app.go:1041`，调用点 L1056，confidence=0.90，import）；`Build`（`internal/boot/boot.go:152`，调用点 L620，confidence=0.90，import）；`subagentEffectiveIdentity`（`internal/boot/boot.go:1454`，调用点 L1473，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `effortNotConfigurableError`（`internal/config/effort.go:327`，调用点 L95/L151，confidence=0.90，exact-match，2 条边）；`ReasoningProtocolForEntry`（`internal/config/effort.go:241`，调用点 L104，confidence=0.90，exact-match）；`explicitReasoningProtocol`（`internal/config/effort.go:257`，调用点 L94，confidence=0.90，exact-match）；`isMiniMaxEntry`（`internal/config/effort.go:289`，调用点 L125，confidence=0.90，exact-match）；`normalizeEffortLevel`（`internal/config/effort.go:347`，调用点 L87，confidence=0.90，exact-match）；`normalizedSupportedEfforts`（`internal/config/effort.go:351`，调用点 L97，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `containsString`（`internal/config/effort.go:338`，调用点 L99，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `App::SetModel`（`desktop/app.go:6021`，调用点 L6022，confidence=0.70，instance-method，待源码复核）；`App::SetEffort`（`desktop/app.go:6155`，调用点 L6156，confidence=0.70，instance-method，待源码复核）；`App::SetSubagentEffort`（`desktop/settings_app.go:1041`，调用点 L1056，confidence=0.90，import，待源码复核）；`buildController`（`internal/automation/runner.go:65`，调用点 L66，confidence=0.90，import，待源码复核）；`Build`（`internal/boot/boot.go:152`，调用点 L620，confidence=0.90，import，待源码复核）；`acpFactory::SessionConfigState`（`internal/cli/acp.go:96`，调用点 L149，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 7 个文件；`internal/config`、`desktop`、`internal/boot`、`internal/cli`、`internal/serve`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/config/edit_test.go`、`internal/config/effort_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/effort.go`、`desktop/app.go`、`desktop/settings_app.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`

#### 待修改项

- [ ] 先冻结 `NormalizeEffort` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L88、L105、L117、L132、L145 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `NormalizeEffort` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/effort.go|NormalizeEffort` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/effort.go:86-153` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `NormalizeEffort` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/config/edit_test.go`、`internal/config/effort_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/effort.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/effort.go|NormalizeEffort` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-180 · 圈复杂度 · `EffortCapabilityForEntry`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-180 |
| Baseline key | `complexity\|internal/config/effort.go\|EffortCapabilityForEntry` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 48） |
| 文件 | `internal/config/effort.go` |
| 代码行 | 39 |
| 函数/方法/包 | `EffortCapabilityForEntry` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`EffortCapabilityForEntry` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 39-82 行；可见控制流热点：L40 `if explicitReasoningProtocol(e) == ReasoningProtocolNone {`；L49 `if def == "" || !containsString(supported, def) {`；L57 `case ReasoningProtocolOpenAI:`；L63 `switch ReasoningProtocolForEntry(e) {`；L69 `switch {`；L77 `case e != nil && e.Kind == "anthropic":`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（resolvedModelReasoningCapability、normalizeEffortLevel、normalizedSupportedEfforts）、持久化/I-O（openAIEffortCapability）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function EffortCapabilityForEntry（internal/config/effort.go:39-82）
- **可信生产调用方（9）：** `App::EffortForTab`（`desktop/app.go:6139`，调用点 L6144，confidence=0.90，import）；`App::runEffortCommandForTab`（`desktop/app.go:6746`，调用点 L6752，confidence=0.90，import）；`subagentEffortLevels`（`internal/boot/boot.go:1361`，调用点 L1368，confidence=0.90，import）；`acpFactory::SessionConfigState`（`internal/cli/acp.go:96`，调用点 L169，confidence=0.90，import）；`chatTUI::currentEffortLevels`（`internal/cli/complete.go:213`，调用点 L225，confidence=0.90，import）；`chatTUI::runEffortCommand`（`internal/cli/effort.go:12`，调用点 L18，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（9）：** `explicitReasoningProtocol`（`internal/config/effort.go:257`，调用点 L40/L54，confidence=0.90，exact-match，2 条边）；`deepSeekEffortCapability`（`internal/config/effort.go:319`，调用点 L56/L65，confidence=0.90，exact-match，2 条边）；`openAIEffortCapability`（`internal/config/effort.go:323`，调用点 L58/L67，confidence=0.90，exact-match，2 条边）；`ReasoningProtocolForEntry`（`internal/config/effort.go:241`，调用点 L63，confidence=0.90，exact-match）；`isMiniMaxEntry`（`internal/config/effort.go:289`，调用点 L70，confidence=0.90，exact-match）；`resolvedModelReasoningCapability`（`internal/config/effort.go:300`，调用点 L60，confidence=0.90，exact-match）；`effortCapabilityFromModel`（`internal/config/effort.go:308`，调用点 L61，confidence=0.90，exact-match）；`normalizeEffortLevel`（`internal/config/effort.go:347`，调用点 L48，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `containsString`（`internal/config/effort.go:338`，调用点 L49，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `App::SlashArgs`（`desktop/app.go:4494`，调用点 L4507，confidence=0.70，instance-method，待源码复核）；`App::Effort`（`desktop/app.go:6135`，调用点 L6136，confidence=0.70，instance-method，待源码复核）；`Build`（`internal/boot/boot.go:152`，调用点 L622，confidence=0.90，import，待源码复核）；`acpFactory::SessionConfigState`（`internal/cli/acp.go:96`，调用点 L169，confidence=0.90，import，待源码复核）；`chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1273，confidence=0.70，instance-method，待源码复核）；`chatREPL`（`internal/cli/cli.go:652`，调用点 L806，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 7 个文件；`internal/config`、`desktop`、`internal/boot`、`internal/cli`、`internal/serve`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/config/edit_test.go`、`internal/config/effort_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/effort.go`、`desktop/app.go`、`internal/boot/boot.go`、`internal/cli/acp.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`

#### 待修改项

- [ ] 先冻结 `EffortCapabilityForEntry` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L40、L54、L60、L66、L77 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `EffortCapabilityForEntry` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/effort.go|EffortCapabilityForEntry` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/effort.go:39-82` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `EffortCapabilityForEntry` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/config/edit_test.go`、`internal/config/effort_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/effort.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/effort.go|EffortCapabilityForEntry` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FL-009 · 文件有效行数 · `internal/config/edit.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-009 |
| Baseline key | `file_lines\|internal/config/edit.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 47） |
| 文件 | `internal/config/edit.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/config/edit.go` |
| Baseline / 当前复测 / 阈值 | 1018 / 1018 / 800 |
| 超出 | +218，1.27x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/config/edit.go` 当前有效代码行 **1018**，超过限制 **800**（+218）。CodeGraph 在文件中索引到的主要大符号包括 mergeTOMLDelta@887-938、Config::RemoveProvider@402-452、replaceTOMLSection@1020-1065、Config::saveProjectIncremental@844-881、Config::SetSkillEnabled@639-665、isUserConfigPath@1195-1218；文件同时承载的调用影响覆盖 15 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 73 个函数/方法节点。
- **可信生产调用方（6）：** `configReasoningLanguageCommand`（`internal/cli/cli.go:1974`，调用点 L2030/L2009/L2028，confidence=0.90，instance-method，3 条边）；`Store::roots`（`internal/skill/skill.go:186`，调用点 L206/L213，confidence=0.90，import，3 条边）；`skillRootsViewFrom`（`desktop/app.go:4932`，调用点 L4963/L4969，confidence=0.90，import，2 条边）；`rootActive`（`desktop/app.go:5064`，调用点 L5065/L5067，confidence=0.90，import，2 条边）；`isConventionSkillRoot`（`desktop/app.go:5195`，调用点 L5196/L5210，confidence=0.90，import，2 条边）；`skillDisplayRoot`（`desktop/app.go:5225`，调用点 L5234/L5237，confidence=0.90，import，2 条边）
- **待源码复核调用方候选（7）：** `chatTUI::runEffortCommand`（`internal/cli/effort.go:12`，调用点 L56/L71/L67/L62，confidence=0.70，instance-method，4 条边，待源码复核）；`App::applyProviderEffortConfig`（`desktop/app.go:6315`，调用点 L6318/L6328/L6323，confidence=0.70，instance-method，3 条边，待源码复核）；`applyEffortEdit`（`internal/serve/serve.go:236`，调用点 L238/L247/L243，confidence=0.70，instance-method，3 条边，待源码复核）；`App::CompleteOnboardingModelSelection`（`desktop/app.go:7820`，调用点 L7847/L7852，confidence=0.70，instance-method，2 条边，待源码复核）；`App::saveDesktopMCPServer`（`desktop/app.go:5642`，调用点 L5651/L5654，confidence=0.70，instance-method，2 条边，待源码复核）；`App::removeDesktopMCPServer`（`desktop/app.go:5661`，调用点 L5669/L5667，confidence=0.70，instance-method，2 条边，待源码复核）
- **可信被调用函数（12）：** `userConfigCandidatePaths`（`internal/config/paths.go:49`，调用点 L1200/L765，confidence=0.90，exact-match，2 条边）；`normalizeProviderEffortFields`（`internal/config/effort.go:220`，调用点 L121，confidence=0.90，exact-match）；`normalizeStoredEffort`（`internal/config/effort.go:230`，调用点 L139，confidence=0.90，exact-match）；`ExpandVars`（`internal/config/expand.go:19`，调用点 L671，confidence=0.90，exact-match）；`NormalizePluginCommandLine`（`internal/config/mcp_command.go:14`，调用点 L695，confidence=0.90，exact-match）；`SourcePath`（`internal/config/paths.go:586`，调用点 L1224，confidence=0.90，exact-match）；`defaultProfilePathResolver`（`internal/config/paths.go:85`，调用点 L1235，confidence=0.90，exact-match）；`AtomicWriteFile`（`internal/fileutil/atomicwrite.go:20`，调用点 L979，confidence=0.90，import）
- **待源码复核被调候选（3）：** `Config::ApplyDeepSeekOfficialDefaultPricing`（`internal/config/pricing.go:72`，调用点 L158/L191，confidence=0.70，instance-method，2 条边，待源码复核）；`ProfilePathResolver::withWorkspaceRoot`（`internal/config/paths.go:166`，调用点 L1240，confidence=0.70，instance-method，待源码复核）；`ProfilePathResolver::UserConfigPath`（`internal/config/paths.go:176`，调用点 L1249，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 15 个文件；`internal/config`、`internal/cli`、`desktop`、`internal/serve`、`internal/skill`、`internal/fileutil`、`internal/mcpdiag`、`internal/netclient`、`internal/permission`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/config/edit_test.go`、`desktop/app_test.go`、`desktop/metrics_app_test.go`、`internal/config/model_fallback_test.go`、`internal/installsource/install_source_test.go`、`internal/cli/chat_tui_test.go`、`desktop/memory_suggestions_test.go`、`desktop/new_session_inherit_test.go`、`desktop/settings_app_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/edit.go`、`internal/cli/effort.go`、`desktop/app.go`、`internal/cli/cli.go`、`internal/serve/serve.go`、`internal/config/paths.go`、`internal/config/pricing.go`、`internal/config/effort.go`、`internal/config/edit_test.go`、`desktop/app_test.go`、`desktop/metrics_app_test.go`、`internal/config/model_fallback_test.go`

#### 待修改项

- [ ] 先冻结 `internal/config/edit.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `mergeTOMLDelta`、`Config::RemoveProvider`、`replaceTOMLSection`、`Config::saveProjectIncremental`、`Config::SetSkillEnabled` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-176、CX-177、CX-178、CX-179。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/config/edit.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/edit.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/config/edit.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/config/edit_test.go`、`desktop/app_test.go`、`desktop/metrics_app_test.go`、`internal/config/model_fallback_test.go`、`internal/installsource/install_source_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 race：`go test -race ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/edit.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/config/edit.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-179 · 圈复杂度 · `replaceTOMLSection`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-179 |
| Baseline key | `complexity\|internal/config/edit.go\|replaceTOMLSection` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 43） |
| 文件 | `internal/config/edit.go` |
| 代码行 | 1020 |
| 函数/方法/包 | `replaceTOMLSection` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`replaceTOMLSection` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 1020-1065 行；可见控制流热点：L1023 `for i, span := range spans {`；L1030 `if arrayIdx >= 0 {`；L1035 `if !ok {`；L1047 `for _, span := range spans {`；L1053 `for _, next := range spans {`；L1057 `if _, _, ok := tomlEditSectionHeader(next.text); ok {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function replaceTOMLSection（internal/config/edit.go:1020-1065）
- **可信生产调用方（2）：** `mergeTOMLDelta`（`internal/config/edit.go:887`，调用点 L935，confidence=0.90，exact-match）；`WritePermissionsSection`（`internal/config/edit.go:992`，调用点 L1012，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `tomlEditSectionHeader`（`internal/config/edit.go:1113`，调用点 L1024/L1034/L1048/L1057，confidence=0.90，exact-match，4 条边）；`tomlLineSpans`（`internal/config/edit.go:1097`，调用点 L1021，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `WritePermissionsSection`（`internal/config/edit.go:992`，调用点 L1012，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/edit.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `replaceTOMLSection` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1023、L1033、L1038、L1053、L1057 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `replaceTOMLSection` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/edit.go|replaceTOMLSection` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/edit.go:1020-1065` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `replaceTOMLSection` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/edit.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/edit.go|replaceTOMLSection` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FL-011 · 文件有效行数 · `internal/config/render.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-011 |
| Baseline key | `file_lines\|internal/config/render.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/config/render.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/config/render.go` |
| Baseline / 当前复测 / 阈值 | 1151 / 1151 / 800 |
| 超出 | +351，1.44x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/config/render.go` 当前有效代码行 **1151**，超过限制 **800**（+351）。CodeGraph 在文件中索引到的主要大符号包括 RenderTOMLForScope@32-569、RenderTOMLProjectDelta@575-946、renderLSPConfig@1069-1109、renderBotSessionMappings@1185-1221、renderPricingMap@956-980、renderBotCredential@1165-1183；文件同时承载的调用影响覆盖 6 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 23 个函数/方法节点。
- **可信生产调用方（1）：** `Config::WriteFile`（`internal/config/config.go:1293`，调用点 L1298，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `normalizeCredentialsStore`（`internal/config/credentials.go:184`，调用点 L60，confidence=0.90，exact-match）；`providerMergeKey`（`internal/config/load.go:351`，调用点 L1046，confidence=0.90，exact-match）；`userConfigDisplayPath`（`internal/config/paths.go:363`，调用点 L48，confidence=0.90，exact-match）
- **待源码复核被调候选（26）：** `Default`（`internal/config/config.go:1230`，调用点 L34/L44/L579/L986，confidence=0.70，exact-match，4 条边，待源码复核）；`Config::UIThemeStyle`（`internal/config/config.go:140`，调用点 L67/L601，confidence=0.70，instance-method，3 条边，待源码复核）；`Config::DesktopCloseBehavior`（`internal/config/config.go:234`，调用点 L78/L102/L608，confidence=0.70，instance-method，3 条边，待源码复核）；`Config::BashMode`（`internal/config/config.go:805`，调用点 L430/L895/L896，confidence=0.70，instance-method，3 条边，待源码复核）；`Config::UITheme`（`internal/config/config.go:127`，调用点 L66/L599，confidence=0.70，instance-method，2 条边，待源码复核）；`Config::UIShortcutLayout`（`internal/config/config.go:147`，调用点 L72/L604，confidence=0.70，instance-method，2 条边，待源码复核）；`Config::NetworkProxyMode`（`internal/config/config.go:656`，调用点 L138/L620，confidence=0.70，instance-method，2 条边，待源码复核）；`Config::ReasoningLanguage`（`internal/config/config.go:396`，调用点 L225/L678，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 524 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/config`、`internal/provider`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/config/render_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/render.go`、`internal/config/config.go`、`internal/config/render_test.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`

#### 待修改项

- [ ] 先冻结 `internal/config/render.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `RenderTOMLForScope`、`RenderTOMLProjectDelta`、`renderLSPConfig`、`renderBotSessionMappings`、`renderPricingMap` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-195、CX-196、CX-197、FN-030、FN-031。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/config/render.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/render.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/config/render.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：并发、取消或事件顺序；持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/config/render_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 race：`go test -race ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/render.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/config/render.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-197 · 圈复杂度 · `isBareTOMLKey`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-197 |
| Baseline key | `complexity\|internal/config/render.go\|isBareTOMLKey` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 32） |
| 文件 | `internal/config/render.go` |
| 代码行 | 1118 |
| 函数/方法/包 | `isBareTOMLKey` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`isBareTOMLKey` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 1118-1129 行；可见控制流热点：L1119 `if key == "" {`；L1122 `for _, r := range key {`；L1123 `if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function isBareTOMLKey（internal/config/render.go:1118-1129）
- **可信生产调用方（1）：** `renderTOMLKeyPart`（`internal/config/render.go:1111`，调用点 L1112，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/render.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `isBareTOMLKey` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1119、L1122、L1123 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `isBareTOMLKey` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/render.go|isBareTOMLKey` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/render.go:1118-1129` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `isBareTOMLKey` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/render.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/render.go|isBareTOMLKey` 下降或消失且无新增/恶化。
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
