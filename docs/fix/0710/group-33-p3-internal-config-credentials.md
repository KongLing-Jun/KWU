# G33 · internal/config · credentials 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G33 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/config` |
| 涉及文件 | `internal/config/config.go`<br>`internal/config/credentials.go`<br>`internal/config/ccswitch.go`<br>`internal/config/mcpjson.go`<br>`internal/config/paths.go` |
| 主要治理目标 | 围绕 `internal/config/credentials.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | config 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/expert-07-08-1614`；涉及核心运行链路 |
| 建议处理顺序 | 总序第 33 组；无同文件组依赖 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；配置、平台或向后兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **43/100**，对象 `internal/config/config.go` 的分项为严重度 4.0、集中度 2.3、影响面 24.9、可执行性 12.0；最高超限 1.28x，文件内 1 条问题，CodeGraph 记录 4 个可信生产调用方、11 个待核候选。

### 治理边界与组间关系

本组涉及 5 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `FL-008` | `file_lines\|internal/config/config.go\|` | 文件有效行数 | `internal/config/config.go` | `internal/config/config.go` | 1 | 1027/800 | +227（1.28x） | P3 | 4 可信 + 11 待核 caller / 16 可信 + 1 待核 callee / 90 unresolved | 待修改 |
| `CX-172` | `complexity\|internal/config/credentials.go\|storeCredentialsInFile` | 圈复杂度 | `internal/config/credentials.go` | `storeCredentialsInFile` | 730 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-171` | `complexity\|internal/config/ccswitch.go\|classifyMCPImportCandidate` | 圈复杂度 | `internal/config/ccswitch.go` | `classifyMCPImportCandidate` | 223 | 16/10 | +6（1.60x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-173` | `complexity\|internal/config/credentials.go\|writeCredentialFileLines` | 圈复杂度 | `internal/config/credentials.go` | `writeCredentialFileLines` | 806 | 11/10 | +1（1.10x） | P3 | 2 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-174` | `complexity\|internal/config/credentials.go\|isCredentialKey` | 圈复杂度 | `internal/config/credentials.go` | `isCredentialKey` | 864 | 11/10 | +1（1.10x） | P3 | 5 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-187` | `complexity\|internal/config/mcpjson.go\|clearMCPJSONAuthentication` | 圈复杂度 | `internal/config/mcpjson.go` | `clearMCPJSONAuthentication` | 329 | 14/10 | +4（1.40x） | P3 | 0 可信 + 0 待核 caller / 5 可信 + 0 待核 callee / 17 unresolved | 待修改 |
| `CX-194` | `complexity\|internal/config/paths.go\|CommandDirsForRoot` | 圈复杂度 | `internal/config/paths.go` | `CommandDirsForRoot` | 548 | 12/10 | +2（1.20x） | P3 | 3 可信 + 0 待核 caller / 5 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-186` | `complexity\|internal/config/mcpjson.go\|loadLegacyMCP` | 圈复杂度 | `internal/config/mcpjson.go` | `loadLegacyMCP` | 105 | 12/10 | +2（1.20x） | P3 | 0 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 7 unresolved | 待修改 |
| `CX-193` | `complexity\|internal/config/paths.go\|cleanEnvDir` | 圈复杂度 | `internal/config/paths.go` | `cleanEnvDir` | 322 | 11/10 | +1（1.10x） | P3 | 3 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-192` | `complexity\|internal/config/paths.go\|cleanExplicitDir` | 圈复杂度 | `internal/config/paths.go` | `cleanExplicitDir` | 136 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### FL-008 · 文件有效行数 · `internal/config/config.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-008 |
| Baseline key | `file_lines\|internal/config/config.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 43） |
| 文件 | `internal/config/config.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/config/config.go` |
| Baseline / 当前复测 / 阈值 | 1027 / 1027 / 800 |
| 超出 | +227，1.28x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/config/config.go` 当前有效代码行 **1027**，超过限制 **800**（+227）。CodeGraph 在文件中索引到的主要大符号包括 Default@1230-1287、IsLikelyChatModel@976-1009、Config::ResolveModel@1321-1354、Config::ResolveModelWithFallback@1360-1387、Config::directProxyHosts@633-653、Config::WriteRootsForRoot@781-800；文件同时承载的调用影响覆盖 17 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 69 个函数/方法节点。
- **可信生产调用方（4）：** `App::Settings`（`desktop/settings_app.go:415`，调用点 L491/L489/L494/L495，confidence=0.90，instance-method，17 条边）；`metricsAggregator::observeSettingsSnapshot`（`desktop/metrics_app.go:217`，调用点 L233/L246/L221/L236，confidence=0.90，instance-method，11 条边）；`desktopStartupSettingsFromConfig`（`desktop/settings_app.go:375`，调用点 L392/L390/L394/L395，confidence=0.90，instance-method，9 条边）；`selectableDesktopModelRef`（`desktop/settings_app.go:1023`，调用点 L1034/L1024/L1031，confidence=0.90，import，3 条边）
- **待源码复核调用方候选（11）：** `Build`（`internal/boot/boot.go:152`，调用点 L186/L613/L1065/L1081，confidence=0.70，instance-method，30 条边，待源码复核）；`RenderTOMLForScope`（`internal/config/render.go:32`，调用点 L34/L44/L395/L391，confidence=0.70，exact-match，27 条边，待源码复核）；`RenderTOMLProjectDelta`（`internal/config/render.go:575`，调用点 L579/L859/L857/L599，confidence=0.70，exact-match，13 条边，待源码复核）；`NormalizeLegacyDesktopProviderAccess`（`internal/config/load.go:927`，调用点 L942/L962/L966/L943，confidence=0.70，instance-method，5 条边，待源码复核）；`providerViewFromEntryForRootWithResolver`（`desktop/settings_app.go:300`，调用点 L317/L304/L314，confidence=0.70，instance-method，4 条边，待源码复核）；`App::ModelsForTab`（`desktop/app.go:5932`，调用点 L5947/L5957/L5954，confidence=0.70，instance-method，3 条边，待源码复核）
- **可信被调用函数（16）：** `Normalize`（`internal/agentmode/mode.go:63`，调用点 L873，confidence=0.90，import）；`auditUserConfigWrite`（`internal/config/audit.go:25`，调用点 L1296，confidence=0.90，exact-match）；`storedCredentialValue`（`internal/config/credentials.go:634`，调用点 L1403，confidence=0.90，exact-match）；`credentialSourceLabel`（`internal/config/credentials.go:541`，调用点 L1436，confidence=0.90，exact-match）；`ResolveCredentialForRootGlobalFirst`（`internal/config/credentials.go:589`，调用点 L1438，confidence=0.90，exact-match）；`renderScopeForPath`（`internal/config/edit.go:1188`，调用点 L1294，confidence=0.90，exact-match）；`configFilePerm`（`internal/config/edit.go:982`，调用点 L1298，confidence=0.90，exact-match）；`desktopProviderAccessMap`（`internal/config/load.go:1034`，调用点 L1325，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Config::expandVars`（`internal/config/expand.go:55`，调用点 L782/L795/L613/L614，confidence=0.70，instance-method，9 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 90 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 17 个文件；`internal/config`、`internal/boot`、`desktop`、`internal/cli`、`internal/agentmode`、`internal/fileutil`、`internal/netclient`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `desktop/app_test.go`、`internal/config/edit_test.go`、`internal/config/render_test.go`、`internal/config/model_fallback_test.go`、`internal/config/backfill_test.go`、`desktop/settings_app_test.go`、`desktop/bot_runtime_app_test.go`、`internal/cli/cli_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/config.go`、`internal/boot/boot.go`、`internal/config/render.go`、`desktop/settings_app.go`、`internal/config/expand.go`、`internal/agentmode/mode.go`、`internal/config/audit.go`、`internal/config/credentials.go`、`desktop/app_test.go`、`internal/config/edit_test.go`、`internal/config/render_test.go`、`internal/config/model_fallback_test.go`

#### 待修改项

- [ ] 先冻结 `internal/config/config.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `Default`、`IsLikelyChatModel`、`Config::ResolveModel`、`Config::ResolveModelWithFallback`、`Config::directProxyHosts` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：无。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/config/config.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/config.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/config/config.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`desktop/app_test.go`、`internal/config/edit_test.go`、`internal/config/render_test.go`、`internal/config/model_fallback_test.go`、`internal/config/backfill_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/config.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/config/config.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-172 · 圈复杂度 · `storeCredentialsInFile`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-172 |
| Baseline key | `complexity\|internal/config/credentials.go\|storeCredentialsInFile` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/config/credentials.go` |
| 代码行 | 730 |
| 函数/方法/包 | `storeCredentialsInFile` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`storeCredentialsInFile` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 730-770 行；可见控制流热点：L731 `if strings.TrimSpace(path) == "" {`；L739 `for _, line := range lines {`；L741 `if _, hit := assignments[key]; hit {`；L751 `if !ok {`；L760 `for key := range assignments {`；L765 `if !replaced[key] {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readCredentialFileLines、writeCredentialFileLines）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function storeCredentialsInFile（internal/config/credentials.go:730-770）
- **可信生产调用方（1）：** `StoreCredentialLinesWithPaths`（`internal/config/credentials.go:348`，调用点 L355，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `readCredentialFileLines`（`internal/config/credentials.go:791`，调用点 L734，confidence=0.90，exact-match）；`writeCredentialFileLines`（`internal/config/credentials.go:806`，调用点 L769，confidence=0.90，exact-match）；`credentialLineKey`（`internal/config/credentials.go:845`，调用点 L750，confidence=0.90，exact-match）；`credentialClearedLineKey`（`internal/config/credentials.go:855`，调用点 L740，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/credentials.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `storeCredentialsInFile` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L731、L740、L749、L760、L765 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `storeCredentialsInFile` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/credentials.go|storeCredentialsInFile` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/credentials.go:730-770` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `storeCredentialsInFile` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/credentials.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/credentials.go|storeCredentialsInFile` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-171 · 圈复杂度 · `classifyMCPImportCandidate`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-171 |
| Baseline key | `complexity\|internal/config/ccswitch.go\|classifyMCPImportCandidate` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 41） |
| 文件 | `internal/config/ccswitch.go` |
| 代码行 | 223 |
| 函数/方法/包 | `classifyMCPImportCandidate` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`classifyMCPImportCandidate` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 223-263 行；可见控制流热点：L228 `if typ == "sse" {`；L236 `if cmd == "npx" || cmd == "uvx" {`；L238 `if strings.Contains(strings.ToLower(a), "@latest") {`；L245 `if len(e.Headers) > 0 || len(e.Env) > 0 {`；L251 `if strings.Contains(name, "flomo") || strings.Contains(name, "dida") ||`；L259 `if len(c.Reasons) == 0 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function classifyMCPImportCandidate（internal/config/ccswitch.go:223-263）
- **可信生产调用方（1）：** `LoadCCSwitchMCPCandidates`（`internal/config/ccswitch.go:36`，调用点 L43，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `isCommonDocMCP`（`internal/config/ccswitch.go:265`，调用点 L247，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/config/ccswitch_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/ccswitch.go`、`internal/config/ccswitch_test.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `classifyMCPImportCandidate` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L228、L236、L245、L251、L259 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `classifyMCPImportCandidate` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/ccswitch.go|classifyMCPImportCandidate` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/ccswitch.go:223-263` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `classifyMCPImportCandidate` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/config/ccswitch_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/ccswitch.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/ccswitch.go|classifyMCPImportCandidate` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-173 · 圈复杂度 · `writeCredentialFileLines`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-173 |
| Baseline key | `complexity\|internal/config/credentials.go\|writeCredentialFileLines` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/config/credentials.go` |
| 代码行 | 806 |
| 函数/方法/包 | `writeCredentialFileLines` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`writeCredentialFileLines` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 806-843 行；可见控制流热点：L807 `if strings.TrimSpace(path) == "" {`；L815 `if dir != "" && dir != "." {`；L816 `if err := os.MkdirAll(dir, 0o700); err != nil {`；L825 `if _, err := tmp.WriteString(out); err != nil {`；L830 `if err := tmp.Close(); err != nil {`；L838 `if err := fileutil.ReplaceFile(tmpPath, path); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（ReplaceFile）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function writeCredentialFileLines（internal/config/credentials.go:806-843）
- **可信生产调用方（2）：** `storeCredentialsInFile`（`internal/config/credentials.go:730`，调用点 L769，confidence=0.90，exact-match）；`removeCredentialFromFile`（`internal/config/credentials.go:772`，调用点 L788，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `ReplaceFile`（`internal/fileutil/atomicwrite.go:67`，调用点 L838，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/config`、`internal/fileutil`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/credentials.go`、`internal/fileutil/atomicwrite.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `writeCredentialFileLines` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L807、L815、L821、L830、L838 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `writeCredentialFileLines` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/credentials.go|writeCredentialFileLines` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/credentials.go:806-843` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `writeCredentialFileLines` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/credentials.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/credentials.go|writeCredentialFileLines` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-174 · 圈复杂度 · `isCredentialKey`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-174 |
| Baseline key | `complexity\|internal/config/credentials.go\|isCredentialKey` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/config/credentials.go` |
| 代码行 | 864 |
| 函数/方法/包 | `isCredentialKey` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`isCredentialKey` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 864-875 行；可见控制流热点：L865 `if key == "" {`；L868 `for i, r := range key {`；L869 `if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9' {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function isCredentialKey（internal/config/credentials.go:864-875）
- **可信生产调用方（5）：** `SetCredentialWithPaths`（`internal/config/credentials.go:366`，调用点 L368，confidence=0.90，exact-match）；`RemoveCredentialWithPaths`（`internal/config/credentials.go:381`，调用点 L384，confidence=0.90，exact-match）；`parseCredentialLines`（`internal/config/credentials.go:452`，调用点 L462，confidence=0.90，exact-match）；`credentialLineKey`（`internal/config/credentials.go:845`，调用点 L852，confidence=0.90，exact-match）；`credentialClearedLineKey`（`internal/config/credentials.go:855`，调用点 L861，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `migrateLegacyCredentialsIfNeededForRoot`（`internal/config/migrate.go:238`，调用点 L259，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/credentials.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `isCredentialKey` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L865、L868、L869 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `isCredentialKey` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/credentials.go|isCredentialKey` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/credentials.go:864-875` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `isCredentialKey` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/credentials.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/credentials.go|isCredentialKey` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-187 · 圈复杂度 · `clearMCPJSONAuthentication`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-187 |
| Baseline key | `complexity\|internal/config/mcpjson.go\|clearMCPJSONAuthentication` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/config/mcpjson.go` |
| 代码行 | 329 |
| 函数/方法/包 | `clearMCPJSONAuthentication` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`clearMCPJSONAuthentication` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 329-382 行；可见控制流热点：L331 `if os.IsNotExist(err) {`；L338 `if err := json.Unmarshal(b, &root); err != nil {`；L346 `if !ok {`；L354 `if !changed {`；L369 `if err != nil {`；L378 `if err := writeMCPJSON(path, root); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（ClearAuthConfig）、状态与控制（setMCPJSONStringMap、setMCPJSONString）、持久化/I-O（writeMCPJSON）、权限/安全（ClearAuthConfig）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function clearMCPJSONAuthentication（internal/config/mcpjson.go:329-382）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `pluginEntryFromMCPSpec`（`internal/config/mcpjson.go:188`，调用点 L355/L381，confidence=0.90，exact-match，2 条边）；`setMCPJSONStringMap`（`internal/config/mcpjson.go:384`，调用点 L365/L366，confidence=0.90，exact-match，2 条边）；`setMCPJSONString`（`internal/config/mcpjson.go:397`，调用点 L367，confidence=0.90，exact-match）；`writeMCPJSON`（`internal/config/mcpjson.go:436`，调用点 L378，confidence=0.90，exact-match）；`ClearAuthConfig`（`internal/mcpdiag/auth.go:78`，调用点 L353，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 17 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/config`、`internal/mcpdiag`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/mcpjson.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `clearMCPJSONAuthentication` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L331、L342、L350、L369、L378 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `clearMCPJSONAuthentication` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/mcpjson.go|clearMCPJSONAuthentication` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/mcpjson.go:329-382` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `clearMCPJSONAuthentication` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容；权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/mcpjson.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/mcpjson.go|clearMCPJSONAuthentication` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-194 · 圈复杂度 · `CommandDirsForRoot`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-194 |
| Baseline key | `complexity\|internal/config/paths.go\|CommandDirsForRoot` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/config/paths.go` |
| 代码行 | 548 |
| 函数/方法/包 | `CommandDirsForRoot` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`CommandDirsForRoot` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 548-583 行；可见控制流热点：L552 `if dir == "" {`；L556 `if samePath(existing, dir) {`；L565 `for _, legacy := range legacyXDGConfigPaths() {`；L568 `if home, err := os.UserHomeDir(); err == nil {`；L573 `if dir := userConfigDir(); dir != "" {`；L579 `for _, dir := range conventionSubdirsAsc(root, "commands") {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（userConfigDir、legacyXDGConfigPaths）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function CommandDirsForRoot（internal/config/paths.go:548-583）
- **可信生产调用方（3）：** `Build`（`internal/boot/boot.go:152`，调用点 L848，confidence=0.90，import）；`CommandDirs`（`internal/config/paths.go:541`，调用点 L542，confidence=0.90，exact-match）；`Controller::ReloadCommands`（`internal/control/controller.go:2914`，调用点 L2920，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `userConfigDir`（`internal/config/paths.go:20`，调用点 L573/L576，confidence=0.90，exact-match，2 条边）；`conventionSubdirsAsc`（`internal/config/paths.go:525`，调用点 L569/L579，confidence=0.90，exact-match，2 条边）；`legacyXDGConfigPaths`（`internal/config/paths.go:288`，调用点 L565，confidence=0.90，exact-match）；`userSupportDir`（`internal/config/paths.go:292`，调用点 L576，confidence=0.90，exact-match）；`legacyOSSupportDir`（`internal/config/paths.go:299`，调用点 L562，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `samePath`（`internal/config/paths.go:345`，调用点 L556/L576，confidence=0.70，exact-match，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** `buildController`（`internal/automation/runner.go:65`，调用点 L66，confidence=0.90，import，待源码复核）；`Build`（`internal/boot/boot.go:152`，调用点 L848，confidence=0.90，import，待源码复核）；`CommandDirs`（`internal/config/paths.go:541`，调用点 L542，confidence=0.90，exact-match，待源码复核）；`Controller::ReloadCommands`（`internal/control/controller.go:2914`，调用点 L2920，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/config`、`internal/boot`、`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/paths.go`、`internal/boot/boot.go`、`internal/control/controller.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `CommandDirsForRoot` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L552、L556、L568、L573、L579 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `CommandDirsForRoot` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/paths.go|CommandDirsForRoot` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/paths.go:548-583` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `CommandDirsForRoot` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/paths.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/paths.go|CommandDirsForRoot` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-186 · 圈复杂度 · `loadLegacyMCP`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-186 |
| Baseline key | `complexity\|internal/config/mcpjson.go\|loadLegacyMCP` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/config/mcpjson.go` |
| 代码行 | 105 |
| 函数/方法/包 | `loadLegacyMCP` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`loadLegacyMCP` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 105-149 行；可见控制流热点：L106 `if path == "" {`；L119 `if err := json.Unmarshal(b, &doc); err != nil {`；L123 `for _, n := range doc.MCPDisabled {`；L131 `for i, raw := range doc.MCP {`；L133 `if !ok || disabled[pe.Name] {`；L141 `if have[pe.Name] {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（NormalizePluginCommandLine、parseLegacyMCPSpec）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function loadLegacyMCP（internal/config/mcpjson.go:105-149）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `NormalizePluginCommandLine`（`internal/config/mcp_command.go:14`，调用点 L145，confidence=0.90，exact-match）；`specsToEntries`（`internal/config/mcpjson.go:73`，调用点 L126，confidence=0.90，exact-match）；`parseLegacyMCPSpec`（`internal/config/mcpjson.go:156`，调用点 L132，confidence=0.90，exact-match）；`anonymousMCPName`（`internal/config/mcpjson.go:184`，调用点 L137，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 7 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/config/mcpjson_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/mcpjson.go`、`internal/config/mcp_command.go`、`internal/config/mcpjson_test.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`

#### 待修改项

- [ ] 先冻结 `loadLegacyMCP` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L106、L119、L128、L133、L141 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `loadLegacyMCP` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/mcpjson.go|loadLegacyMCP` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/mcpjson.go:105-149` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `loadLegacyMCP` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/config/mcpjson_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/mcpjson.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/mcpjson.go|loadLegacyMCP` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-193 · 圈复杂度 · `cleanEnvDir`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-193 |
| Baseline key | `complexity\|internal/config/paths.go\|cleanEnvDir` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/config/paths.go` |
| 代码行 | 322 |
| 函数/方法/包 | `cleanEnvDir` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`cleanEnvDir` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 322-343 行；可见控制流热点：L324 `if dir == "" {`；L328 `if dir == "~" {`；L329 `if home, err := os.UserHomeDir(); err == nil && home != "" {`；L333 `if home, err := os.UserHomeDir(); err == nil && home != "" {`；L337 `if !filepath.IsAbs(dir) {`；L338 `if abs, err := filepath.Abs(dir); err == nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function cleanEnvDir（internal/config/paths.go:322-343）
- **可信生产调用方（3）：** `suiyuanHomeDir`（`internal/config/paths.go:24`，调用点 L25，confidence=0.90，exact-match）；`userSupportDir`（`internal/config/paths.go:292`，调用点 L293，confidence=0.90，exact-match）；`userCacheDir`（`internal/config/paths.go:303`，调用点 L304，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `ExpandVars`（`internal/config/expand.go:19`，调用点 L327，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `mcpGlobalMigrationMarkerPath`（`internal/config/migrate.go:162`，调用点 L163，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/paths.go`、`internal/config/expand.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `cleanEnvDir` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L324、L328、L333、L337、L338 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `cleanEnvDir` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/paths.go|cleanEnvDir` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/paths.go:322-343` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `cleanEnvDir` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/paths.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/paths.go|cleanEnvDir` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-192 · 圈复杂度 · `cleanExplicitDir`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-192 |
| Baseline key | `complexity\|internal/config/paths.go\|cleanExplicitDir` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/config/paths.go` |
| 代码行 | 136 |
| 函数/方法/包 | `cleanExplicitDir` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`cleanExplicitDir` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 136-157 行；可见控制流热点：L138 `if dir == "" {`；L142 `if dir == "~" {`；L143 `if home, err := os.UserHomeDir(); err == nil && home != "" {`；L147 `if home, err := os.UserHomeDir(); err == nil && home != "" {`；L151 `if !filepath.IsAbs(dir) {`；L152 `if abs, err := filepath.Abs(dir); err == nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function cleanExplicitDir（internal/config/paths.go:136-157）
- **可信生产调用方（1）：** `NewProfilePathResolverWithOptions`（`internal/config/paths.go:89`，调用点 L95/L99/L107，confidence=0.90，exact-match，3 条边）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `ExpandVars`（`internal/config/expand.go:19`，调用点 L141，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/paths.go`、`internal/config/expand.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `cleanExplicitDir` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L138、L142、L147、L151、L152 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `cleanExplicitDir` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/paths.go|cleanExplicitDir` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/paths.go:136-157` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `cleanExplicitDir` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/fix-cloud-account-auth-form-binding`、`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/paths.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/paths.go|cleanExplicitDir` 下降或消失且无新增/恶化。
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
