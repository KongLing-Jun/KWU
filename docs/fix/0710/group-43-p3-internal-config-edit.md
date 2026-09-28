# G43 · internal/config · edit 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G43 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/config`、`internal/memory`、`internal/profile` |
| 涉及文件 | `internal/config/edit.go`<br>`internal/config/dotenv.go`<br>`internal/memory/recall.go`<br>`internal/memory/store.go`<br>`internal/profile/policy.go` |
| 主要治理目标 | 围绕 `internal/memory/store.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | config / memory / profile 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 6 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 43 组；等待 G04、G37 的同文件变更合并 |
| 前置依赖 | G04（internal/config/edit.go）；G37（internal/memory/store.go） |
| 后续验证重点 | 协议、序列化或流式事件兼容；配置、平台或向后兼容；持久化、迁移或文件格式兼容；权限、安全或凭据边界；会话、状态或恢复语义；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **39/100**，对象 `*Config.RemoveProvider` 的分项为严重度 6.6、集中度 5.9、影响面 14.5、可执行性 12.0；最高超限 1.70x，文件内 5 条问题，CodeGraph 记录 0 个可信生产调用方、1 个待核候选。

### 治理边界与组间关系

本组涉及 5 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G04、G37。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-176` | `complexity\|internal/config/edit.go\|*Config.RemoveProvider` | 圈复杂度 | `internal/config/edit.go` | `*Config.RemoveProvider` | 402 | 17/10 | +7（1.70x） | P3 | 0 可信 + 1 待核 caller / 0 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-178` | `complexity\|internal/config/edit.go\|mergeTOMLDelta` | 圈复杂度 | `internal/config/edit.go` | `mergeTOMLDelta` | 887 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-175` | `complexity\|internal/config/dotenv.go\|loadDotEnvFileAs` | 圈复杂度 | `internal/config/dotenv.go` | `loadDotEnvFileAs` | 90 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-177` | `complexity\|internal/config/edit.go\|*Config.saveProjectIncremental` | 圈复杂度 | `internal/config/edit.go` | `*Config.saveProjectIncremental` | 844 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 5 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-295` | `complexity\|internal/memory/recall.go\|searchMemories` | 圈复杂度 | `internal/memory/recall.go` | `searchMemories` | 106 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 9 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-297` | `complexity\|internal/memory/store.go\|Store.Save` | 圈复杂度 | `internal/memory/store.go` | `Store.Save` | 229 | 11/10 | +1（1.10x） | P3 | 0 可信 + 0 待核 caller / 6 可信 + 5 待核 callee / 2 unresolved | 待修改 |
| `CX-298` | `complexity\|internal/memory/store.go\|Store.Archive` | 圈复杂度 | `internal/memory/store.go` | `Store.Archive` | 271 | 12/10 | +2（1.20x） | P3 | 0 可信 + 0 待核 caller / 4 可信 + 3 待核 callee / 2 unresolved | 待修改 |
| `CX-334` | `complexity\|internal/profile/policy.go\|Policy.ResolveLoading` | 圈复杂度 | `internal/profile/policy.go` | `Policy.ResolveLoading` | 58 | 11/10 | +1（1.10x） | P3 | 0 可信 + 0 待核 caller / 3 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-335` | `complexity\|internal/profile/policy.go\|Policy.DecideScope` | 圈复杂度 | `internal/profile/policy.go` | `Policy.DecideScope` | 132 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-299` | `complexity\|internal/memory/store.go\|Store.List` | 圈复杂度 | `internal/memory/store.go` | `Store.List` | 564 | 13/10 | +3（1.30x） | P3 | 0 可信 + 0 待核 caller / 1 可信 + 2 待核 callee / 9 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-176 · 圈复杂度 · `*Config.RemoveProvider`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-176 |
| Baseline key | `complexity\|internal/config/edit.go\|*Config.RemoveProvider` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/config/edit.go` |
| 代码行 | 402 |
| 函数/方法/包 | `*Config.RemoveProvider` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Config.RemoveProvider` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 402-452 行；可见控制流热点：L405 `for i := range c.Providers {`；L411 `if idx < 0 {`；L420 `if c.modelRefTargetsProvider(ref, name) {`；L435 `if defaultRefsProvider {`；L441 `if subagentRefsProvider {`；L445 `if fallback != "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Config::modelRefTargetsProvider、Config::providerRemovalFallback）、网络/协议（Config::modelRefTargetsProvider、Config::providerRemovalFallback）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Config::RemoveProvider（internal/config/edit.go:402-452）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `App::deleteProviderAndRetargetTabs`（`desktop/settings_app.go:1510`，调用点 L1551，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（2）：** `Config::modelRefTargetsProvider`（`internal/config/edit.go:388`，调用点 L415/L416/L417/L420，confidence=0.70，instance-method，4 条边，待源码复核）；`Config::providerRemovalFallback`（`internal/config/edit.go:454`，调用点 L427，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `App::DeleteProvider`（`desktop/settings_app.go:1366`，调用点 L1367，confidence=0.70，instance-method，待源码复核）；`App::RemoveProviderAccess`（`desktop/settings_app.go:1375`，调用点 L1387，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/config`、`desktop`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/config/edit_test.go`、`internal/config/model_fallback_test.go`
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/edit.go`、`desktop/settings_app.go`、`internal/config/edit_test.go`、`internal/config/model_fallback_test.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`

#### 待修改项

- [ ] 先冻结 `*Config.RemoveProvider` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L405、L419、L429、L438、L445 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Config.RemoveProvider` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/edit.go|*Config.RemoveProvider` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/edit.go:402-452` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Config.RemoveProvider` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/config/edit_test.go`、`internal/config/model_fallback_test.go`。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/edit.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/edit.go|*Config.RemoveProvider` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-178 · 圈复杂度 · `mergeTOMLDelta`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-178 |
| Baseline key | `complexity\|internal/config/edit.go\|mergeTOMLDelta` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/config/edit.go` |
| 代码行 | 887 |
| 函数/方法/包 | `mergeTOMLDelta` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`mergeTOMLDelta` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 887-938 行；可见控制流热点：L901 `if curName == "" {`；L905 `if curIsArray && len(sections) > 0 && sections[len(sections)-1].isArray && sections[len(sections)-1…`；L914 `if name, isArray, ok := tomlEditSectionHeader(line); ok {`；L921 `if curName != "" {`；L931 `if top := strings.TrimSpace(topLevel.String()); top != "" {`；L934 `for _, s := range sections {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function mergeTOMLDelta（internal/config/edit.go:887-938）
- **可信生产调用方（1）：** `Config::saveProjectIncremental`（`internal/config/edit.go:844`，调用点 L875，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `mergeTOMLTopLevelFields`（`internal/config/edit.go:940`，调用点 L932，confidence=0.90，exact-match）；`replaceTOMLSection`（`internal/config/edit.go:1020`，调用点 L935，confidence=0.90，exact-match）；`tomlEditSectionHeader`（`internal/config/edit.go:1113`，调用点 L914，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/edit.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `mergeTOMLDelta` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L901、L913、L921、L925、L934 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `mergeTOMLDelta` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/edit.go|mergeTOMLDelta` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/edit.go:887-938` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `mergeTOMLDelta` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/edit.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/edit.go|mergeTOMLDelta` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-175 · 圈复杂度 · `loadDotEnvFileAs`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-175 |
| Baseline key | `complexity\|internal/config/dotenv.go\|loadDotEnvFileAs` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/config/dotenv.go` |
| 代码行 | 90 |
| 函数/方法/包 | `loadDotEnvFileAs` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`loadDotEnvFileAs` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 90-122 行；可见控制流热点：L92 `if err != nil {`；L98 `for sc.Scan() {`；L100 `if line == "" || strings.HasPrefix(line, "#") {`；L110 `if key == "" {`；L113 `if _, exists := os.LookupEnv(key); exists && source.Kind != CredentialSourceCredentials {`；L117 `if err := os.Setenv(key, val); err == nil && source.Kind != "" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function loadDotEnvFileAs（internal/config/dotenv.go:90-122）
- **可信生产调用方（1）：** `loadCredentialStoreForRootWithPaths`（`internal/config/credentials.go:327`，调用点 L336，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `recordExistingCredentialSource`（`internal/config/credentials.go:481`，调用点 L114，confidence=0.90，exact-match）；`recordCredentialSource`（`internal/config/credentials.go:501`，调用点 L119，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/dotenv.go`、`internal/config/credentials.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `loadDotEnvFileAs` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L92、L100、L105、L113、L117 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `loadDotEnvFileAs` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/dotenv.go|loadDotEnvFileAs` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/dotenv.go:90-122` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `loadDotEnvFileAs` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/dotenv.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/dotenv.go|loadDotEnvFileAs` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-177 · 圈复杂度 · `*Config.saveProjectIncremental`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-177 |
| Baseline key | `complexity\|internal/config/edit.go\|*Config.saveProjectIncremental` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/config/edit.go` |
| 代码行 | 844 |
| 函数/方法/包 | `*Config.saveProjectIncremental` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Config.saveProjectIncremental` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 844-881 行；可见控制流热点：L845 `if strings.TrimSpace(path) == "" {`；L850 `if err != nil {`；L860 `if isNew {`；L865 `if tomlBodyHasTopLevelKey(body, "config_version") && !tomlBodyHasTopLevelKey(delta, "config_version…`；L874 `if strings.TrimSpace(delta) != "" {`；L877 `if removePlugins {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（writeConfigFile）、状态与控制（tomlBodyHasTopLevelKey）、持久化/I-O（writeConfigFile）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Config::saveProjectIncremental（internal/config/edit.go:844-881）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Config::SaveTo`（`internal/config/edit.go:809`，调用点 L815，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（5）：** `writeConfigFile`（`internal/config/edit.go:971`，调用点 L861/L880，confidence=0.90，exact-match，2 条边）；`tomlBodyHasTopLevelKey`（`internal/config/edit.go:1166`，调用点 L865，confidence=0.90，exact-match，2 条边）；`mergeTOMLDelta`（`internal/config/edit.go:887`，调用点 L875，confidence=0.90，exact-match）；`removeTOMLSection`（`internal/config/edit.go:1067`，调用点 L878，confidence=0.90，exact-match）；`tomlBodyHasSection`（`internal/config/edit.go:1178`，调用点 L868，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `App::CompleteOnboardingModelSelection`（`desktop/app.go:7820`，调用点 L7852，confidence=0.70，instance-method，待源码复核）；`App::SetAutoPlan`（`desktop/settings_app.go:1066`，调用点 L1077，confidence=0.70，instance-method，待源码复核）；`App::SetMemoryCompilerEnabled`（`desktop/settings_app.go:1096`，调用点 L1104，confidence=0.70，instance-method，待源码复核）；`App::SetReasoningLanguage`（`desktop/settings_app.go:1960`，调用点 L1971，confidence=0.70，instance-method，待源码复核）；`Config::Save`（`internal/config/edit.go:1223`，调用点 L1228，confidence=0.70，instance-method，待源码复核）；`migrateMCPToUserConfig`（`internal/config/migrate.go:105`，调用点 L155，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`、`internal/config/configured_test.go`、`internal/config/credentials_test.go`、`internal/config/default_test.go`、`internal/config/dotenv_test.go`、`internal/config/edit_test.go`、`internal/config/effort_test.go`、`internal/config/expand_test.go`
- **需同步检查：** `internal/config/edit.go`、`internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`

#### 待修改项

- [ ] 先冻结 `*Config.saveProjectIncremental` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L845、L851、L865、L869、L877 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Config.saveProjectIncremental` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/config/edit.go|*Config.saveProjectIncremental` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/config/edit.go:844-881` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Config.saveProjectIncremental` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/config/audit_test.go`、`internal/config/backfill_test.go`、`internal/config/ccswitch_test.go`、`internal/config/chat_model_test.go`、`internal/config/commanddirs_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/config`。
- [ ] 运行相关包 vet：`go vet ./internal/config`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/config/edit.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/config/edit.go|*Config.saveProjectIncremental` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-295 · 圈复杂度 · `searchMemories`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-295 |
| Baseline key | `complexity\|internal/memory/recall.go\|searchMemories` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/memory/recall.go` |
| 代码行 | 106 |
| 函数/方法/包 | `searchMemories` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`searchMemories` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 106-172 行；可见控制流热点：L108 `if query == "" {`；L117 `for _, m := range memories {`；L123 `if len(terms) == 0 {`；L139 `for _, doc := range docs {`；L149 `if score <= 0 {`；L168 `if len(hits) > limit {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（ArchiveSink::Err、Store::List）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function searchMemories（internal/memory/recall.go:106-172）
- **可信生产调用方（1）：** `recallTool::Execute`（`internal/memory/recall.go:48`，调用点 L69，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（9）：** `filterMemories`（`internal/memory/recall.go:185`，调用点 L115，confidence=0.90，exact-match）；`memorySearchText`（`internal/memory/recall.go:211`，调用点 L121，confidence=0.90，exact-match）；`Tokens`（`internal/retrieval/bm25.go:14`，调用点 L122，confidence=0.90，import）；`Counts`（`internal/retrieval/bm25.go:58`，调用点 L130，confidence=0.90，import）；`BM25Score`（`internal/retrieval/bm25.go:67`，调用点 L148，confidence=0.90，import）；`DocumentFrequency`（`internal/retrieval/bm25.go:97`，调用点 L143，confidence=0.90，import）；`KeepTopRelativeScore`（`internal/retrieval/bm25.go:111`，调用点 L165，confidence=0.90，import）；`QueryTerms`（`internal/retrieval/bm25.go:131`，调用点 L111，confidence=0.90，import）
- **待源码复核被调候选（2）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L118，confidence=0.70，instance-method，待源码复核）；`Store::List`（`internal/checkpoint/checkpoint.go:205`，调用点 L115，confidence=0.80，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `recallTool::Execute`（`internal/memory/recall.go:48`，调用点 L69，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/memory`、`internal/retrieval`、`internal/automation`、`internal/checkpoint`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`、`internal/memory/quickadd_test.go`、`internal/memory/recall_test.go`、`internal/memory/remember_test.go`、`internal/memory/store_extra_test.go`、`internal/memory/store_test.go`、`internal/memory/writedoc_test.go`
- **需同步检查：** `internal/memory/recall.go`、`internal/retrieval/bm25.go`、`internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`、`internal/memory/quickadd_test.go`

#### 待修改项

- [ ] 逐段核对 L108、L118、L134、L149、L168 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `searchMemories` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memory/recall.go|searchMemories` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memory/recall.go:106-172` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `searchMemories` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`、`internal/memory/quickadd_test.go`、`internal/memory/recall_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memory`。
- [ ] 运行相关包 vet：`go vet ./internal/memory`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memory/recall.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memory/recall.go|searchMemories` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-297 · 圈复杂度 · `Store.Save`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-297 |
| Baseline key | `complexity\|internal/memory/store.go\|Store.Save` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/memory/store.go` |
| 代码行 | 229 |
| 函数/方法/包 | `Store.Save` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Store.Save` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 229-263 行；可见控制流热点：L231 `if dir == "" {`；L238 `if err := ensureMemoryDir(dir); err != nil {`；L245 `if err := writeMemoryFile(path, []byte(render(m, name))); err != nil {`；L248 `if err := reindexIn(dir, name, m); err != nil {`；L252 `if sameDir(otherDir, dir) {`；L258 `if err := removeActiveMemoryInDir(otherDir, name); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（writeMemoryFile、Store::DirFor、Store::dirs、Store::hiddenMemoryInDir）、渲染/输出（render）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::Save（internal/memory/store.go:229-263）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `sameDir`（`internal/memory/doc.go:272`，调用点 L252，confidence=0.90，exact-match）；`removeActiveMemoryInDir`（`internal/memory/store.go:303`，调用点 L258，confidence=0.90，exact-match）；`safeJoin`（`internal/memory/store.go:394`，调用点 L241，confidence=0.90，exact-match）；`ensureMemoryDir`（`internal/memory/store.go:450`，调用点 L238，confidence=0.90，exact-match）；`writeMemoryFile`（`internal/memory/store.go:460`，调用点 L245，confidence=0.90，exact-match）；`reindexIn`（`internal/memory/store.go:554`，调用点 L248，confidence=0.90，exact-match）
- **待源码复核被调候选（5）：** `Store::DirFor`（`internal/memory/store.go:116`，调用点 L230，confidence=0.70，instance-method，待源码复核）；`Store::dirs`（`internal/memory/store.go:136`，调用点 L251，confidence=0.70，instance-method，待源码复核）；`Store::hiddenMemoryInDir`（`internal/memory/store.go:317`，调用点 L255，confidence=0.70，instance-method，待源码复核）；`render`（`internal/memory/store.go:487`，调用点 L245，confidence=0.70，exact-match，待源码复核）；`slug`（`internal/memory/store.go:702`，调用点 L234，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 2 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/memory`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`、`internal/memory/quickadd_test.go`、`internal/memory/recall_test.go`、`internal/memory/remember_test.go`、`internal/memory/store_extra_test.go`、`internal/memory/store_test.go`、`internal/memory/writedoc_test.go`
- **需同步检查：** `internal/memory/store.go`、`internal/memory/doc.go`、`internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`、`internal/memory/quickadd_test.go`

#### 待修改项

- [ ] 逐段核对 L231、L238、L248、L252、L258 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Store.Save` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memory/store.go|Store.Save` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memory/store.go:229-263` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Store.Save` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`、`internal/memory/quickadd_test.go`、`internal/memory/recall_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memory`。
- [ ] 运行相关包 vet：`go vet ./internal/memory`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memory/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memory/store.go|Store.Save` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-298 · 圈复杂度 · `Store.Archive`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-298 |
| Baseline key | `complexity\|internal/memory/store.go\|Store.Archive` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/memory/store.go` |
| 代码行 | 271 |
| 函数/方法/包 | `Store.Archive` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Store.Archive` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 271-301 行；可见控制流热点：L272 `if s.Dir == "" && s.GlobalDir == "" {`；L280 `for _, dir := range s.dirs() {`；L281 `if dir == "" {`；L288 `if err != nil {`；L291 `if p != "" || indexContainsIn(dir, name) {`；L296 `if p != "" {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（archiveInDir、Store::dirs、Store::hiddenMemoryInDir）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::Archive（internal/memory/store.go:271-301）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `archiveInDir`（`internal/memory/store.go:339`，调用点 L287，confidence=0.90，exact-match）；`indexLinesExceptIn`（`internal/memory/store.go:510`，调用点 L292，confidence=0.90，exact-match）；`indexContainsIn`（`internal/memory/store.go:521`，调用点 L291，confidence=0.90，exact-match）；`flushIndexIn`（`internal/memory/store.go:536`，调用点 L292，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `Store::dirs`（`internal/memory/store.go:136`，调用点 L280，confidence=0.70，instance-method，待源码复核）；`Store::hiddenMemoryInDir`（`internal/memory/store.go:317`，调用点 L284，confidence=0.70，instance-method，待源码复核）；`slug`（`internal/memory/store.go:702`，调用点 L275，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 2 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memory`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `desktop/app_test.go`、`internal/memory/store_test.go`、`internal/profile/store_test.go`
- **同目录测试候选：** `internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`、`internal/memory/quickadd_test.go`、`internal/memory/recall_test.go`、`internal/memory/remember_test.go`、`internal/memory/store_extra_test.go`、`internal/memory/writedoc_test.go`
- **需同步检查：** `internal/memory/store.go`、`desktop/app_test.go`、`internal/memory/store_test.go`、`internal/profile/store_test.go`、`internal/memory/convention_test.go`

#### 待修改项

- [ ] 逐段核对 L272、L280、L284、L291、L296 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Store.Archive` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memory/store.go|Store.Archive` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memory/store.go:271-301` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Store.Archive` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`desktop/app_test.go`、`internal/memory/store_test.go`、`internal/profile/store_test.go`。
- [ ] 运行相关包测试：`go test ./internal/memory`。
- [ ] 运行相关包 vet：`go vet ./internal/memory`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memory/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memory/store.go|Store.Archive` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-334 · 圈复杂度 · `Policy.ResolveLoading`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-334 |
| Baseline key | `complexity\|internal/profile/policy.go\|Policy.ResolveLoading` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/profile/policy.go` |
| 代码行 | 58 |
| 函数/方法/包 | `Policy.ResolveLoading` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Policy.ResolveLoading` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 58-84 行；可见控制流热点：L61 `for _, item := range items {`；L63 `if key == "" || !p.LoadingEnabled(item.Scope) || !loadableStatus(item.Status) {`；L66 `if keySetHas(opts.CurrentUserKeys, key) || keySetHas(opts.ProjectMemoryKeys, key) {`；L70 `if exists {`；L71 `if existing.Scope == ScopeProject || item.Scope != ScopeProject {`；L80 `for _, key := range order {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadableStatus、Policy::LoadingEnabled）、状态与控制（keySetHas）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Policy::ResolveLoading（internal/profile/policy.go:58-84）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `keySetHas`（`internal/profile/policy.go:201`，调用点 L66，confidence=0.90，exact-match，2 条边）；`loadableStatus`（`internal/profile/policy.go:197`，调用点 L63，confidence=0.90，exact-match）；`itemKey`（`internal/profile/policy.go:217`，调用点 L62，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Policy::LoadingEnabled`（`internal/profile/policy.go:51`，调用点 L63，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/profile/policy_test.go`
- **同目录测试候选：** `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`、`internal/profile/path_test.go`、`internal/profile/selector_test.go`、`internal/profile/service_test.go`、`internal/profile/store_test.go`、`internal/profile/worker_test.go`
- **需同步检查：** `internal/profile/policy.go`、`internal/profile/policy_test.go`、`internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`

#### 待修改项

- [ ] 逐段核对 L61、L63、L70、L71、L80 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Policy.ResolveLoading` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/profile/policy.go|Policy.ResolveLoading` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/profile/policy.go:58-84` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Policy.ResolveLoading` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/profile/policy_test.go`。
- [ ] 运行相关包测试：`go test ./internal/profile`。
- [ ] 运行相关包 vet：`go vet ./internal/profile`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/profile/policy.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/profile/policy.go|Policy.ResolveLoading` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-335 · 圈复杂度 · `Policy.DecideScope`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-335 |
| Baseline key | `complexity\|internal/profile/policy.go\|Policy.DecideScope` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/profile/policy.go` |
| 代码行 | 132 |
| 函数/方法/包 | `Policy.DecideScope` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Policy.DecideScope` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 132-151 行；可见控制流热点：L133 `if validScope(c.SuggestedScope) {`；L137 `if strings.TrimSpace(evt.WorkspaceRoot) != "" {`；L138 `if strings.Contains(text, "project") ||`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Policy::DecideScope（internal/profile/policy.go:132-151）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Policy::PrepareCandidate`（`internal/profile/policy.go:86`，调用点 L92，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `validScope`（`internal/profile/model.go:140`，调用点 L133，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`、`internal/profile/path_test.go`、`internal/profile/policy_test.go`、`internal/profile/selector_test.go`、`internal/profile/service_test.go`、`internal/profile/store_test.go`、`internal/profile/worker_test.go`
- **需同步检查：** `internal/profile/policy.go`、`internal/profile/model.go`、`internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`

#### 待修改项

- [ ] 逐段核对 L133、L137、L138 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `Policy.DecideScope` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/profile/policy.go|Policy.DecideScope` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/profile/policy.go:132-151` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Policy.DecideScope` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/profile`。
- [ ] 运行相关包 vet：`go vet ./internal/profile`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/profile/policy.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/profile/policy.go|Policy.DecideScope` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-299 · 圈复杂度 · `Store.List`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-299 |
| Baseline key | `complexity\|internal/memory/store.go\|Store.List` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 32） |
| 文件 | `internal/memory/store.go` |
| 代码行 | 564 |
| 函数/方法/包 | `Store.List` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Store.List` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 564-595 行；可见控制流热点：L565 `if s.Dir == "" && s.GlobalDir == "" {`；L571 `if dir == "" {`；L575 `if err != nil {`；L579 `if e.IsDir() || e.Name() == indexFile || !strings.HasSuffix(e.Name(), ".md") {`；L582 `if m, ok := loadMemory(filepath.Join(dir, e.Name())); ok {`；L586 `if !seen[m.Name] {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadMemory）、持久化/I-O（Store::dirs、Store::visibleFromDir）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::List（internal/memory/store.go:564-595）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `loadMemory`（`internal/memory/store.go:673`，调用点 L582，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Store::dirs`（`internal/memory/store.go:136`，调用点 L570，confidence=0.70，instance-method，待源码复核）；`Store::visibleFromDir`（`internal/memory/store.go:648`，调用点 L583，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 9 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memory`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`、`internal/memory/quickadd_test.go`、`internal/memory/recall_test.go`、`internal/memory/remember_test.go`、`internal/memory/store_extra_test.go`、`internal/memory/store_test.go`、`internal/memory/writedoc_test.go`
- **需同步检查：** `internal/memory/store.go`、`internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`、`internal/memory/quickadd_test.go`

#### 待修改项

- [ ] 逐段核对 L565、L571、L578、L582、L586 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Store.List` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memory/store.go|Store.List` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memory/store.go:564-595` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Store.List` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`、`internal/memory/quickadd_test.go`、`internal/memory/recall_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/memory`。
- [ ] 运行相关包 vet：`go vet ./internal/memory`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memory/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memory/store.go|Store.List` 下降或消失且无新增/恶化。
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
