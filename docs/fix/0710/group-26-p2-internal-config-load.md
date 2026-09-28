# G26 · internal/config · load 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G26 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/config` |
| 涉及文件 | `internal/config/load.go`<br>`internal/config/migrate.go` |
| 主要治理目标 | 围绕 `internal/config/load.go` 等治理对象降低复杂度/体量，统一处理 1 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | config 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`；单文件最多集中 6 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 26 组；无同文件组依赖 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；配置、平台或向后兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **45/100**，对象 `internal/config/load.go` 的分项为严重度 3.9、集中度 6.4、影响面 26.6、可执行性 8.0；最高超限 1.26x，文件内 6 条问题，CodeGraph 记录 13 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 2 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `FL-010` | `file_lines\|internal/config/load.go\|` | 文件有效行数 | `internal/config/load.go` | `internal/config/load.go` | 1 | 1005/800 | +205（1.26x） | P2 | 13 可信 + 0 待核 caller / 28 可信 + 12 待核 callee / 139 unresolved | 待修改 |
| `CX-188` | `complexity\|internal/config/migrate.go\|migrateMCPToUserConfig` | 圈复杂度 | `internal/config/migrate.go` | `migrateMCPToUserConfig` | 105 | 16/10 | +6（1.60x） | P2 | 0 可信 + 0 待核 caller / 7 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-183` | `complexity\|internal/config/load.go\|mergeTOMLProviders` | 圈复杂度 | `internal/config/load.go` | `mergeTOMLProviders` | 303 | 11/10 | +1（1.10x） | P2 | 1 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 8 unresolved | 待修改 |
| `ND-010` | `nesting_depth\|internal/config/load.go\|mergeTOMLProviders` | 嵌套深度 | `internal/config/load.go` | `mergeTOMLProviders` | 303 | 5/4 | +1（1.25x） | P2 | 1 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 8 unresolved | 待修改 |
| `CX-189` | `complexity\|internal/config/migrate.go\|migrateLegacyCredentialsIfNeededForRoot` | 圈复杂度 | `internal/config/migrate.go` | `migrateLegacyCredentialsIfNeededForRoot` | 238 | 12/10 | +2（1.20x） | P3 | 0 可信 + 0 待核 caller / 9 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-191` | `complexity\|internal/config/migrate.go\|legacyPlugins` | 圈复杂度 | `internal/config/migrate.go` | `legacyPlugins` | 398 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-190` | `complexity\|internal/config/migrate.go\|migrateLegacyTOMLIfNeeded` | 圈复杂度 | `internal/config/migrate.go` | `migrateLegacyTOMLIfNeeded` | 323 | 12/10 | +2（1.20x） | P3 | 0 可信 + 0 待核 caller / 2 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-182` | `complexity\|internal/config/load.go\|LoadForRootWithPaths` | 圈复杂度 | `internal/config/load.go` | `LoadForRootWithPaths` | 42 | 12/10 | +2（1.20x） | P3 | 2 可信 + 0 待核 caller / 22 可信 + 5 待核 callee / 4 unresolved | 待修改 |
| `CX-185` | `complexity\|internal/config/load.go\|NormalizeLegacyDesktopProviderAccess` | 圈复杂度 | `internal/config/load.go` | `NormalizeLegacyDesktopProviderAccess` | 927 | 15/10 | +5（1.50x） | P3 | 1 可信 + 0 待核 caller / 4 可信 + 3 待核 callee / 12 unresolved | 待修改 |
| `CX-184` | `complexity\|internal/config/load.go\|applyLegacyMimoCatalog` | 圈复杂度 | `internal/config/load.go` | `applyLegacyMimoCatalog` | 640 | 13/10 | +3（1.30x） | P3 | 1 可信 + 0 待核 caller / 5 可信 + 1 待核 callee / 7 unresolved | 待修改 |

## 3. 逐条治理任务

### FL-010 · 文件有效行数 · `internal/config/load.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-010 |
| Baseline key | `file_lines\|internal/config/load.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 45） |
| 文件 | `internal/config/load.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/config/load.go` |
| Baseline / 当前复测 / 阈值 | 1005 / 1005 / 800 |
| 超出 | +205，1.26x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/config/load.go` 当前有效代码行 **1005**，超过限制 **800**（+205）。CodeGraph 在文件中索引到的主要大符号包括 LoadForRootWithPaths@42-127、NormalizeLegacyDesktopProviderAccess@927-975、mergeTOMLProviders@303-342、applyLegacyMimoCatalog@640-679、backfillDeepSeekPro@173-211、legacyMimoCustomProvider@859-894；文件同时承载的调用影响覆盖 17 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 67 个函数/方法节点。
- **可信生产调用方（13）：** `App::loadDesktopUserConfigForView`（`desktop/settings_app.go:705`，调用点 L718/L733/L712/L725，confidence=0.90，import，4 条边）；`App::loadDesktopUserConfigForEditWithPaths`（`desktop/settings_app.go:671`，调用点 L677/L686/L694，confidence=0.90，import，3 条边）；`App::buildTabControllerWithContext`（`desktop/tabs.go:2144`，调用点 L2167/L2227/L2250，confidence=0.90，import，3 条边）；`normalizeDesktopProviderAccessList`（`internal/config/suiyuan_official.go:83`，调用点 L91/L101/L84，confidence=0.90，exact-match，3 条边）；`ensureSuiyuanOfficialProvider`（`internal/config/suiyuan_official.go:119`，调用点 L121/L122/L127，confidence=0.90，exact-match，3 条边）；`App::cachedSkillRootsView`（`desktop/app.go:4887`，调用点 L4895/L4894，confidence=0.90，import，2 条边）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（28）：** `mimoDomesticPrices`（`internal/config/pricing.go:109`，调用点 L663/L665/L872/L886，confidence=0.90，exact-match，4 条边）；`deepSeekV4PricesForConfig`（`internal/config/pricing.go:51`，调用点 L1061/L1065/L217，confidence=0.90，exact-match，3 条边）；`loadCredentialStoreForRootWithPaths`（`internal/config/credentials.go:327`，调用点 L46/L495，confidence=0.90，exact-match，2 条边）；`credentialsStoreModeWithPaths`（`internal/config/credentials.go:199`，调用点 L49/L123，confidence=0.90，exact-match，2 条边）；`isUserConfigPath`（`internal/config/edit.go:1195`，调用点 L487/L345，confidence=0.90，exact-match，2 条边）；`normalizeEffortConfig`（`internal/config/effort.go:211`，调用点 L120/L481，confidence=0.90，exact-match，2 条边）；`normalizePluginCommandLines`（`internal/config/mcp_command.go:33`，调用点 L111/L471，confidence=0.90，exact-match，2 条边）；`defaultProfilePathResolver`（`internal/config/paths.go:85`，调用点 L31/L151，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（12）：** `ProviderEntry::ModelList`（`internal/config/config.go:952`，调用点 L962/L966/L656/L181，confidence=0.70，instance-method，7 条边，待源码复核）；`Config::Provider`（`internal/config/config.go:1302`，调用点 L1046/L1063/L785/L996，confidence=0.70，instance-method，5 条边，待源码复核）；`Default`（`internal/config/config.go:1230`，调用点 L47/L443/L156/L464，confidence=0.70，exact-match，4 条边，待源码复核）；`ProfilePathResolver::withWorkspaceRoot`（`internal/config/paths.go:166`，调用点 L44/L155/L403/L424，confidence=0.70，instance-method，4 条边，待源码复核）；`ProviderEntry::Configured`（`internal/config/config.go:1459`，调用点 L943/L966，confidence=0.70，instance-method，2 条边，待源码复核）；`ProfilePathResolver::UserConfigLoadPath`（`internal/config/paths.go:183`，调用点 L57/L157，confidence=0.70，instance-method，2 条边，待源码复核）；`ProfilePathResolver::UserConfigPath`（`internal/config/paths.go:176`，调用点 L404/L425，confidence=0.70，instance-method，2 条边，待源码复核）；`Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L942，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 139 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 17 个文件；`internal/config`、`desktop`、`internal/boot`、`internal/cli`、`internal/serve`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/mcp_test.go`、`desktop/app_test.go`、`internal/cli/chat_tui_test.go`、`internal/config/backfill_test.go`、`internal/control/controller_test.go`、`desktop/settings_app_test.go`、`internal/boot/boot_test.go`、`internal/cli/cli_test.go`、`internal/config/edit_test.go`、`internal/config/loadedit_test.go`、`internal/config/render_test.go`、`internal/serve/modelswitch_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/load.go`、`desktop/settings_app.go`、`desktop/tabs.go`、`internal/config/suiyuan_official.go`、`internal/config/config.go`、`internal/config/paths.go`、`internal/cli/mcp_test.go`、`desktop/app_test.go`、`internal/cli/chat_tui_test.go`、`internal/config/backfill_test.go`

#### 待修改项

- [ ] 先冻结 `internal/config/load.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `LoadForRootWithPaths`、`NormalizeLegacyDesktopProviderAccess`、`mergeTOMLProviders`、`applyLegacyMimoCatalog`、`backfillDeepSeekPro` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-182、CX-183、CX-184、CX-185、ND-010。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/config/load.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/load.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/config/load.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/mcp_test.go`、`desktop/app_test.go`、`internal/cli/chat_tui_test.go`、`internal/config/backfill_test.go`、`internal/control/controller_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/load.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/config/load.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-188 · 圈复杂度 · `migrateMCPToUserConfig`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-188 |
| Baseline key | `complexity\|internal/config/migrate.go\|migrateMCPToUserConfig` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 45） |
| 文件 | `internal/config/migrate.go` |
| 代码行 | 105 |
| 函数/方法/包 | `migrateMCPToUserConfig` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`migrateMCPToUserConfig` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 105-160 行；可见控制流热点：L107 `if dest == "" {`；L115 `for _, p := range userCfg.Plugins {`；L127 `for _, entry := range entries {`；L140 `for _, path := range mcpMigrationLegacyTOMLPaths(dest, home) {`；L151 `if result.Sources == 0 {`；L155 `if err := userCfg.SaveTo(dest); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadPluginEntriesFromTOML、validatePlugin、NormalizePluginCommandLine、loadLegacyConfigPlugins）、持久化/I-O（Config::SaveTo）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function migrateMCPToUserConfig（internal/config/migrate.go:105-160）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（7）：** `loadPluginEntriesFromTOML`（`internal/config/migrate.go:181`，调用点 L141/L144，confidence=0.90，exact-match，2 条边）；`validatePlugin`（`internal/config/edit.go:781`，调用点 L130，confidence=0.90，exact-match）；`NormalizePluginCommandLine`（`internal/config/mcp_command.go:14`，调用点 L128，confidence=0.90，exact-match）；`mcpMigrationLegacyTOMLPaths`（`internal/config/migrate.go:170`，调用点 L140，confidence=0.90，exact-match）；`loadLegacyConfigPlugins`（`internal/config/migrate.go:201`，调用点 L149，confidence=0.90，exact-match）；`normalizedMCPMigrationRoots`（`internal/config/migrate.go:217`，调用点 L143，confidence=0.90，exact-match）；`userConfigPath`（`internal/config/paths.go:12`，调用点 L106，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Config::SaveTo`（`internal/config/edit.go:809`，调用点 L155，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/migrate.go`、`internal/config/edit.go`、`internal/config/mcp_command.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `migrateMCPToUserConfig` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L107、L116、L130、L145、L155 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `migrateMCPToUserConfig` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/migrate.go|migrateMCPToUserConfig` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/migrate.go:105-160` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `migrateMCPToUserConfig` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/migrate.go|migrateMCPToUserConfig` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-183 · 圈复杂度 · `mergeTOMLProviders`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-183 |
| Baseline key | `complexity\|internal/config/load.go\|mergeTOMLProviders` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 44） |
| 文件 | `internal/config/load.go` |
| 代码行 | 303 |
| 函数/方法/包 | `mergeTOMLProviders` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | `ND-010` |

#### 明确的问题描述

`mergeTOMLProviders` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 303-342 行；可见控制流热点：L309 `for _, path := range paths {`；L310 `if _, err := os.Stat(path); err != nil {`；L314 `if _, err := toml.DecodeFile(path, &f); err != nil {`；L322 `for _, p := range f.Providers {`；L325 `if i, ok := index[key]; ok {`；L326 `if sources[key] == providerSourceProject && source == providerSourceUser {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeProviderEffortFields）、网络/协议（normalizeProviderEffortFields、providerSourceForPath、providerMergeKey）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function mergeTOMLProviders（internal/config/load.go:303-342）
- **可信生产调用方（1）：** `LoadForRootWithPaths`（`internal/config/load.go:42`，调用点 L85，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `normalizeProviderEffortFields`（`internal/config/effort.go:220`，调用点 L323，confidence=0.90，exact-match）；`providerSourceForPath`（`internal/config/load.go:344`，调用点 L321，confidence=0.90，exact-match）；`providerMergeKey`（`internal/config/load.go:351`，调用点 L324，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 8 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/load.go`、`internal/config/effort.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `mergeTOMLProviders` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L309、L314、L317、L325、L326 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `mergeTOMLProviders` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-183、ND-010 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/load.go|mergeTOMLProviders` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/load.go:303-342` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `mergeTOMLProviders` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-183`、`ND-010` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/load.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/load.go|mergeTOMLProviders` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-010 · 嵌套深度 · `mergeTOMLProviders`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-010 |
| Baseline key | `nesting_depth\|internal/config/load.go\|mergeTOMLProviders` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 44） |
| 文件 | `internal/config/load.go` |
| 代码行 | 303 |
| 函数/方法/包 | `mergeTOMLProviders` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-183` |

#### 明确的问题描述

`mergeTOMLProviders` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L309 `for _, path := range paths {`；L310 `if _, err := os.Stat(path); err != nil {`；L314 `if _, err := toml.DecodeFile(path, &f); err != nil {`；L322 `for _, p := range f.Providers {`；L325 `if i, ok := index[key]; ok {`；L326 `if sources[key] == providerSourceProject && source == providerSourceUser {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function mergeTOMLProviders（internal/config/load.go:303-342）
- **可信生产调用方（1）：** `LoadForRootWithPaths`（`internal/config/load.go:42`，调用点 L85，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `normalizeProviderEffortFields`（`internal/config/effort.go:220`，调用点 L323，confidence=0.90，exact-match）；`providerSourceForPath`（`internal/config/load.go:344`，调用点 L321，confidence=0.90，exact-match）；`providerMergeKey`（`internal/config/load.go:351`，调用点 L324，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 8 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/load.go`、`internal/config/effort.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `mergeTOMLProviders` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 从 L309、L314、L317、L325、L326 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-183、ND-010 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/config/load.go|mergeTOMLProviders` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/load.go:303-342` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `mergeTOMLProviders` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-183`、`ND-010` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/load.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/config/load.go|mergeTOMLProviders` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-189 · 圈复杂度 · `migrateLegacyCredentialsIfNeededForRoot`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-189 |
| Baseline key | `complexity\|internal/config/migrate.go\|migrateLegacyCredentialsIfNeededForRoot` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/config/migrate.go` |
| 代码行 | 238 |
| 函数/方法/包 | `migrateLegacyCredentialsIfNeededForRoot` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`migrateLegacyCredentialsIfNeededForRoot` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 238-271 行；可见控制流热点：L243 `for _, key := range credentialEnvNamesForRoot(root) {`；L247 `if value, ok := legacyKeyringCredentialValueLookup(key); ok {`；L251 `for _, src := range legacyCredentialsPaths() {`；L256 `if err != nil {`；L260 `for key, value := range assignments {`；L266 `if len(missing) == 0 {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseCredentialLines）、持久化/I-O（StoreCredentialLines、credentialCurrentStoreHasKey、credentialCurrentStoreClearedKey）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function migrateLegacyCredentialsIfNeededForRoot（internal/config/migrate.go:238-271）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（9）：** `walkIgnorer::skip`（`internal/tool/builtin/gitignore.go:84`，调用点 L244/L261，confidence=0.90，exact-match，2 条边）；`legacyKeyringCredentialValueLookup`（`internal/config/credentials.go:121`，调用点 L247，confidence=0.90，exact-match）；`credentialEnvNamesForRoot`（`internal/config/credentials.go:213`，调用点 L243，confidence=0.90，exact-match）；`StoreCredentialLines`（`internal/config/credentials.go:342`，调用点 L269，confidence=0.90，exact-match）；`credentialCurrentStoreHasKey`（`internal/config/credentials.go:423`，调用点 L241，confidence=0.90，exact-match）；`credentialCurrentStoreClearedKey`（`internal/config/credentials.go:431`，调用点 L241，confidence=0.90，exact-match）；`parseCredentialLines`（`internal/config/credentials.go:452`，调用点 L259，confidence=0.90，exact-match）；`legacyCredentialsPaths`（`internal/config/dotenv.go:60`，调用点 L251，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/config`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/migrate.go`、`internal/tool/builtin/gitignore.go`、`internal/config/credentials.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `migrateLegacyCredentialsIfNeededForRoot` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L243、L247、L252、L260、L266 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `migrateLegacyCredentialsIfNeededForRoot` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/migrate.go|migrateLegacyCredentialsIfNeededForRoot` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/migrate.go:238-271` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `migrateLegacyCredentialsIfNeededForRoot` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/migrate.go|migrateLegacyCredentialsIfNeededForRoot` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-191 · 圈复杂度 · `legacyPlugins`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-191 |
| Baseline key | `complexity\|internal/config/migrate.go\|legacyPlugins` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/config/migrate.go` |
| 代码行 | 398 |
| 函数/方法/包 | `legacyPlugins` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`legacyPlugins` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 398-449 行；可见控制流热点：L400 `for _, n := range legacy.MCPDisabled {`；L406 `if off {`；L418 `for i, raw := range legacy.MCP {`；L420 `if !ok {`；L431 `for n := range legacy.MCPServers {`；L435 `for _, name := range names {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（NormalizePluginCommandLine、normalizeTransport）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function legacyPlugins（internal/config/migrate.go:398-449）
- **可信生产调用方（1）：** `loadLegacyConfigPlugins`（`internal/config/migrate.go:201`，调用点 L214，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `NormalizePluginCommandLine`（`internal/config/mcp_command.go:14`，调用点 L410，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `mergeEnv`（`internal/config/migrate.go:473`，调用点 L426/L442，confidence=0.70，exact-match，2 条边，待源码复核）；`normalizeTransport`（`internal/config/migrate.go:453`，调用点 L439，confidence=0.70，exact-match，待源码复核）；`firstNonEmpty`（`internal/config/migrate.go:464`，调用点 L439，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `migrateMCPToUserConfig`（`internal/config/migrate.go:105`，调用点 L149，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/migrate.go`、`internal/config/mcp_command.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `legacyPlugins` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L400、L411、L420、L423、L435 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `legacyPlugins` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/migrate.go|legacyPlugins` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/migrate.go:398-449` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `legacyPlugins` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/migrate.go|legacyPlugins` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-190 · 圈复杂度 · `migrateLegacyTOMLIfNeeded`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-190 |
| Baseline key | `complexity\|internal/config/migrate.go\|migrateLegacyTOMLIfNeeded` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/config/migrate.go` |
| 代码行 | 323 |
| 函数/方法/包 | `migrateLegacyTOMLIfNeeded` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`migrateLegacyTOMLIfNeeded` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 323-356 行；可见控制流热点：L324 `for _, src := range legacyTOMLPaths(dest, home) {`；L328 `if _, err := os.Stat(src); err != nil {`；L332 `if err := mergeFile(cfg, src); err != nil {`；L339 `if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {`；L342 `if err := cfg.WriteFile(dest); err != nil {`；L349 `if warnings := migrateSupportData(legacyDir, newDir); len(warnings) > 0 {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（migrateSupportData）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function migrateLegacyTOMLIfNeeded（internal/config/migrate.go:323-356）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `legacyTOMLPaths`（`internal/config/migrate.go:358`，调用点 L324，confidence=0.90，exact-match）；`migrateSupportData`（`internal/config/migrate.go:502`，调用点 L349，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `samePath`（`internal/config/paths.go:345`，调用点 L348，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/migrate.go`、`internal/config/paths.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `migrateLegacyTOMLIfNeeded` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L324、L328、L336、L342、L349 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `migrateLegacyTOMLIfNeeded` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/migrate.go|migrateLegacyTOMLIfNeeded` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/migrate.go:323-356` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `migrateLegacyTOMLIfNeeded` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/migrate.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/migrate.go|migrateLegacyTOMLIfNeeded` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-182 · 圈复杂度 · `LoadForRootWithPaths`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-182 |
| Baseline key | `complexity\|internal/config/load.go\|LoadForRootWithPaths` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/config/load.go` |
| 代码行 | 42 |
| 函数/方法/包 | `LoadForRootWithPaths` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`LoadForRootWithPaths` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 42-127 行；可见控制流热点：L52 `if root != "." {`；L59 `if err := mergeRuntimeTOMLFile(cfg, uc); err != nil {`；L68 `if err := mergeRuntimeTOMLFile(cfg, projectTOML); err != nil {`；L85 `if providers, providerSources, shadowedProjectProviders, ok, err := mergeTOMLProviders(tomlSources)…`；L92 `if access, ok, err := mergeTOMLProviderAccess(tomlSources); err != nil {`；L106 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（resolveProviderCredentialsForRootWithPaths、loadCredentialStoreForRootWithPaths、loadProjectDotEnvForExpansion、normalizeEffortConfig）、状态与控制（mergeRuntimeTOMLFile、Config::setExpansionEnv）、持久化/I-O（credentialsStoreModeWithPaths、mergeRuntimeTOMLFile、loadCredentialStoreForRootWithPaths、ProfilePathResolver::withWorkspaceRoot）、网络/协议（resolveProviderCredentialsForRootWithPaths、mergeTOMLProviders、mergeTOMLProviderAccess、normalizeLegacyProviderModels）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function LoadForRootWithPaths（internal/config/load.go:42-127）
- **可信生产调用方（2）：** `LoadForRoot`（`internal/config/load.go:30`，调用点 L31，confidence=0.90，exact-match）；`LoadForRootWithActor`（`internal/config/load.go:34`，调用点 L39，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（22）：** `credentialsStoreModeWithPaths`（`internal/config/credentials.go:199`，调用点 L49/L123，confidence=0.90，exact-match，2 条边）；`mergeRuntimeTOMLFile`（`internal/config/load.go:516`，调用点 L59/L68，confidence=0.90，exact-match，2 条边）；`resolveProviderCredentialsForRootWithPaths`（`internal/config/credentials.go:275`，调用点 L125，confidence=0.90，exact-match）；`loadCredentialStoreForRootWithPaths`（`internal/config/credentials.go:327`，调用点 L46，confidence=0.90，exact-match）；`loadProjectDotEnvForExpansion`（`internal/config/dotenv.go:29`，调用点 L45，confidence=0.90，exact-match）；`normalizeEffortConfig`（`internal/config/effort.go:211`，调用点 L120，confidence=0.90，exact-match）；`userAutoPlanModeWithPaths`（`internal/config/load.go:154`，调用点 L122，confidence=0.90，exact-match）；`backfillDeepSeekPro`（`internal/config/load.go:173`，调用点 L121，confidence=0.90，exact-match）
- **待源码复核被调候选（5）：** `Config::setExpansionEnv`（`internal/config/load.go:129`，调用点 L48/L124，confidence=0.70，instance-method，2 条边，待源码复核）；`Default`（`internal/config/config.go:1230`，调用点 L47，confidence=0.70，exact-match，待源码复核）；`Config::mergeMCPJSON`（`internal/config/mcpjson.go:209`，调用点 L109，confidence=0.70，instance-method，待源码复核）；`ProfilePathResolver::withWorkspaceRoot`（`internal/config/paths.go:166`，调用点 L44，confidence=0.70，instance-method，待源码复核）；`ProfilePathResolver::UserConfigLoadPath`（`internal/config/paths.go:183`，调用点 L57，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `App::NeedsOnboarding`（`desktop/app.go:7686`，调用点 L7692，confidence=0.90，import，待源码复核）；`Build`（`internal/boot/boot.go:152`，调用点 L170，confidence=0.90，import，待源码复核）；`chatTUI::applyMCPMode`（`internal/cli/mcp_manager_actions.go:134`，调用点 L140，confidence=0.90，import，待源码复核）；`Controller::DisabledSkills`（`internal/control/controller.go:2961`，调用点 L2962，confidence=0.90，import，待源码复核）；`Controller::SkillEnabled`（`internal/control/controller.go:2976`，调用点 L2977，confidence=0.90，import，待源码复核）；`Server::index`（`internal/serve/serve.go:338`，调用点 L346，confidence=0.90，import，待源码复核）
- **未解析调用：** 4 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 9 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/load.go`、`internal/config/credentials.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `LoadForRootWithPaths` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L52、L59、L81、L92、L106 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `LoadForRootWithPaths` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/load.go|LoadForRootWithPaths` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/load.go:42-127` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `LoadForRootWithPaths` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容；权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/load.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/load.go|LoadForRootWithPaths` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-185 · 圈复杂度 · `NormalizeLegacyDesktopProviderAccess`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-185 |
| Baseline key | `complexity\|internal/config/load.go\|NormalizeLegacyDesktopProviderAccess` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/config/load.go` |
| 代码行 | 927 |
| 函数/方法/包 | `NormalizeLegacyDesktopProviderAccess` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`NormalizeLegacyDesktopProviderAccess` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 927-975 行；可见控制流热点：L928 `if c == nil || len(c.Desktop.ProviderAccess) > 0 {`；L942 `if entry, ok := c.ResolveModel(ref); ok {`；L953 `for _, ref := range c.Agent.SubagentModels {`；L957 `for _, conn := range c.Bot.Connections {`；L962 `if legacyMimoProviderName(p.Name) != "" && len(p.ModelList()) > 0 {`；L970 `if len(access) == 0 {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeDesktopOfficialProviderAccess、desktopProviderAccessNameForConfig、ProviderEntry::Configured、Config::ResolveModel）、网络/协议（legacyMimoProviderName、normalizeDesktopOfficialProviderAccess、desktopProviderAccessNameForConfig、desktopProviderAccessMap）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function NormalizeLegacyDesktopProviderAccess（internal/config/load.go:927-975）
- **可信生产调用方（1）：** `normalizeLegacyDesktopProviderAccessForSettings`（`desktop/settings_app.go:809`，调用点 L813，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `legacyMimoProviderName`（`internal/config/load.go:818`，调用点 L962，confidence=0.90，exact-match）；`normalizeDesktopOfficialProviderAccess`（`internal/config/load.go:896`，调用点 L974，confidence=0.90，exact-match）；`desktopProviderAccessNameForConfig`（`internal/config/load.go:986`，调用点 L934，confidence=0.90，exact-match）；`desktopProviderAccessMap`（`internal/config/load.go:1034`，调用点 L931，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `ProviderEntry::ModelList`（`internal/config/config.go:952`，调用点 L962/L966，confidence=0.70，instance-method，2 条边，待源码复核）；`ProviderEntry::Configured`（`internal/config/config.go:1459`，调用点 L943/L966，confidence=0.70，instance-method，2 条边，待源码复核）；`Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L942，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 12 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/config`、`desktop`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/config/backfill_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/load.go`、`desktop/settings_app.go`、`internal/config/backfill_test.go`、`internal/config/audit_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `NormalizeLegacyDesktopProviderAccess` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L928、L942、L957、L962、L970 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `NormalizeLegacyDesktopProviderAccess` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/load.go|NormalizeLegacyDesktopProviderAccess` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/load.go:927-975` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `NormalizeLegacyDesktopProviderAccess` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/config/backfill_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/load.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/load.go|NormalizeLegacyDesktopProviderAccess` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-184 · 圈复杂度 · `applyLegacyMimoCatalog`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-184 |
| Baseline key | `complexity\|internal/config/load.go\|applyLegacyMimoCatalog` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/config/load.go` |
| 代码行 | 640 |
| 函数/方法/包 | `applyLegacyMimoCatalog` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`applyLegacyMimoCatalog` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 640-679 行；可见控制流热点：L641 `if p == nil || len(models) == 0 {`；L653 `if currentDefault == "" {`；L662 `if p.Prices == nil {`；L665 `for model, price := range mimoDomesticPrices(models) {`；L666 `if p.Prices[model] == nil {`。CodeGraph 记录的下游职责候选主要涉及 网络/协议（ProviderEntry::ModelList）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function applyLegacyMimoCatalog（internal/config/load.go:640-679）
- **可信生产调用方（1）：** `normalizeLegacyMimoProviderCatalogs`（`internal/config/load.go:616`，调用点 L628/L632，confidence=0.90，exact-match，2 条边）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `stringSlicesEqual`（`internal/config/load.go:681`，调用点 L672/L673，confidence=0.90，exact-match，2 条边）；`mergeModelLists`（`internal/config/load.go:1115`，调用点 L656/L659，confidence=0.90，exact-match，2 条边）；`mimoDomesticPrices`（`internal/config/pricing.go:109`，调用点 L663/L665，confidence=0.90，exact-match，2 条边）；`backfillOfficialContextWindow`（`internal/config/load.go:1092`，调用点 L660，confidence=0.90，exact-match）；`firstKnownModel`（`internal/config/load.go:1131`，调用点 L658，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `ProviderEntry::ModelList`（`internal/config/config.go:952`，调用点 L656，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 7 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/load.go`、`internal/config/pricing.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `applyLegacyMimoCatalog` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L641、L653、L662、L665、L666 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `applyLegacyMimoCatalog` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/load.go|applyLegacyMimoCatalog` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/load.go:640-679` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `applyLegacyMimoCatalog` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/load.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/load.go|applyLegacyMimoCatalog` 下降或消失且无新增/恶化。
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
