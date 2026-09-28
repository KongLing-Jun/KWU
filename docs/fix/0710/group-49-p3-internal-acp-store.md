# G49 · internal/acp · store 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G49 |
| 问题数量 | 4 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/acp`、`internal/automation`、`internal/bot` |
| 涉及文件 | `internal/automation/store.go`<br>`internal/bot/gateway.go`<br>`internal/acp/service.go` |
| 主要治理目标 | 围绕 `internal/acp/service.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | acp / automation / bot 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/main-v2-start-20260630`、`origin/codex/im-session-refresh`；单文件最多集中 10 条问题 |
| 建议处理顺序 | 总序第 49 组；等待 G29、G34、G41、G46、G48 的同文件变更合并 |
| 前置依赖 | G29（internal/bot/gateway.go）；G34（internal/acp/service.go）；G41（internal/acp/service.go）；G46（internal/automation/store.go）；G48（internal/acp/service.go） |
| 后续验证重点 | 会话、状态或恢复语义；权限、安全或凭据边界 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **26/100**，对象 `*Store.DeleteTaskTemplate` 的分项为严重度 3.6、集中度 4.6、影响面 6.2、可执行性 12.0；最高超限 1.40x，文件内 3 条问题，CodeGraph 记录 0 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 3 个文件，按相近得分与模块/调用域补足本组 4 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G29、G34、G41、G46、G48。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-075` | `complexity\|internal/automation/store.go\|*Store.DeleteTaskTemplate` | 圈复杂度 | `internal/automation/store.go` | `*Store.DeleteTaskTemplate` | 158 | 14/10 | +4（1.40x） | P3 | 0 可信 + 0 待核 caller / 1 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-076` | `complexity\|internal/automation/store.go\|*Store.RestoreTaskTemplate` | 圈复杂度 | `internal/automation/store.go` | `*Store.RestoreTaskTemplate` | 212 | 14/10 | +4（1.40x） | P3 | 0 可信 + 0 待核 caller / 1 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-087` | `complexity\|internal/bot/gateway.go\|*BotGateway.sessionOptionsForMessage` | 圈复杂度 | `internal/bot/gateway.go` | `*BotGateway.sessionOptionsForMessage` | 988 | 12/10 | +2（1.20x） | P3 | 0 可信 + 2 待核 caller / 1 可信 + 1 待核 callee / 4 unresolved | 待修改 |
| `CX-018` | `complexity\|internal/acp/service.go\|isSafeSessionID` | 圈复杂度 | `internal/acp/service.go` | `isSafeSessionID` | 1456 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-075 · 圈复杂度 · `*Store.DeleteTaskTemplate`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-075 |
| Baseline key | `complexity\|internal/automation/store.go\|*Store.DeleteTaskTemplate` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 26） |
| 文件 | `internal/automation/store.go` |
| 代码行 | 158 |
| 函数/方法/包 | `*Store.DeleteTaskTemplate` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Store.DeleteTaskTemplate` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 158-209 行；可见控制流热点：L160 `if id == "" {`；L168 `if err := s.ensureTemplatesDB(db); err != nil {`；L179 `if err != nil && err != sql.ErrNoRows {`；L183 `if _, err := tx.Exec(\`UPDATE task_templates SET deleted = 1, updated_at = ? WHERE id = ?\`, time.Now…`；L191 `if isDefaultTaskTemplateID(id) {`；L203 `if err := tx.Commit(); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（Store::openTemplatesDB、Store::ensureTemplatesDB、Store::Begin）、任务/并发（isDefaultTaskTemplateID）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::DeleteTaskTemplate（internal/automation/store.go:158-209）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `isDefaultTaskTemplateID`（`internal/automation/store.go:439`，调用点 L191，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `Store::openTemplatesDB`（`internal/automation/store.go:321`，调用点 L163，confidence=0.70，instance-method，待源码复核）；`Store::ensureTemplatesDB`（`internal/automation/store.go:336`，调用点 L168，confidence=0.70，instance-method，待源码复核）；`Store::Begin`（`internal/checkpoint/checkpoint.go:102`，调用点 L171，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/automation`、`internal/checkpoint`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/automation/store_test.go`
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/store.go`、`internal/checkpoint/checkpoint.go`、`internal/automation/store_test.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`

#### 待修改项

- [ ] 先冻结 `*Store.DeleteTaskTemplate` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L160、L172、L182、L191、L203 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Store.DeleteTaskTemplate` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/store.go|*Store.DeleteTaskTemplate` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/store.go:158-209` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Store.DeleteTaskTemplate` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/main-v2-start-20260630` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/automation/store_test.go`。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/store.go|*Store.DeleteTaskTemplate` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-076 · 圈复杂度 · `*Store.RestoreTaskTemplate`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-076 |
| Baseline key | `complexity\|internal/automation/store.go\|*Store.RestoreTaskTemplate` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 26） |
| 文件 | `internal/automation/store.go` |
| 代码行 | 212 |
| 函数/方法/包 | `*Store.RestoreTaskTemplate` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Store.RestoreTaskTemplate` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 212-262 行；可见控制流热点：L215 `if err != nil {`；L223 `if err != nil {`；L233 `if err := tx.Commit(); err != nil {`；L239 `if _, err := tx.Exec(\`UPDATE task_templates SET deleted = 0, updated_at = ? WHERE id = ?\`, now, id)…`；L252 `if err != nil {`；L258 `if err := tx.Commit(); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（Store::openTemplatesDB、Store::ensureTemplatesDB、Store::Begin）、任务/并发（isDefaultTaskTemplateID）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::RestoreTaskTemplate（internal/automation/store.go:212-262）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `isDefaultTaskTemplateID`（`internal/automation/store.go:439`，调用点 L238，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `Store::openTemplatesDB`（`internal/automation/store.go:321`，调用点 L214，confidence=0.70，instance-method，待源码复核）；`Store::ensureTemplatesDB`（`internal/automation/store.go:336`，调用点 L219，confidence=0.70，instance-method，待源码复核）；`Store::Begin`（`internal/checkpoint/checkpoint.go:102`，调用点 L222，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/automation`、`internal/checkpoint`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/automation/store_test.go`
- **同目录测试候选：** `internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`、`internal/automation/business_test.go`、`internal/automation/condition_test.go`、`internal/automation/engine_test.go`、`internal/automation/export_test.go`、`internal/automation/lease_test.go`、`internal/automation/migration_heartbeat_test.go`、`internal/automation/migration_test.go`、`internal/automation/model_test.go`、`internal/automation/prompt_test.go`
- **需同步检查：** `internal/automation/store.go`、`internal/checkpoint/checkpoint.go`、`internal/automation/store_test.go`、`internal/automation/architecture_test.go`、`internal/automation/archive_sink_test.go`、`internal/automation/archive_test.go`

#### 待修改项

- [ ] 先冻结 `*Store.RestoreTaskTemplate` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L215、L229、L238、L248、L258 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Store.RestoreTaskTemplate` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/automation/store.go|*Store.RestoreTaskTemplate` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/automation/store.go:212-262` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Store.RestoreTaskTemplate` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/main-v2-start-20260630` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/automation/store_test.go`。
- [ ] 运行相关包测试：`go test ./internal/automation`。
- [ ] 运行相关包 vet：`go vet ./internal/automation`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/automation/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/automation/store.go|*Store.RestoreTaskTemplate` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-087 · 圈复杂度 · `*BotGateway.sessionOptionsForMessage`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-087 |
| Baseline key | `complexity\|internal/bot/gateway.go\|*BotGateway.sessionOptionsForMessage` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 26） |
| 文件 | `internal/bot/gateway.go` |
| 代码行 | 988 |
| 函数/方法/包 | `*BotGateway.sessionOptionsForMessage` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*BotGateway.sessionOptionsForMessage` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 988-1023 行；可见控制流热点：L992 `if gw.cfg.ConnectionChannels != nil && msg.ConnectionID != "" {`；L994 `if value := strings.TrimSpace(channel.Model); value != "" {`；L1000 `if value := normalizeOptionalBotToolApprovalMode(channel.ToolApprovalMode); value != "" {`；L1006 `if gw.cfg.Channels == nil {`；L1013 `if value := strings.TrimSpace(channel.Model); value != "" {`；L1019 `if value := normalizeOptionalBotToolApprovalMode(channel.ToolApprovalMode); value != "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeOptionalBotToolApprovalMode、normalizeBotToolApprovalMode）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method BotGateway::sessionOptionsForMessage（internal/bot/gateway.go:988-1023）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `BotGateway::currentToolApprovalMode`（`internal/bot/gateway.go:722`，调用点 L732，confidence=0.70，instance-method，待源码复核）；`BotGateway::getOrCreateSession`（`internal/bot/gateway.go:848`，调用点 L860，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `normalizeOptionalBotToolApprovalMode`（`internal/bot/gateway.go:1032`，调用点 L1000/L1019，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（1）：** `normalizeBotToolApprovalMode`（`internal/bot/gateway.go:1025`，调用点 L991，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `BotGateway::runTurn`（`internal/bot/gateway.go:763`，调用点 L782，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 4 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/bot/gateway_test.go`
- **同目录测试候选：** `internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/qq/gateway_test.go`、`internal/bot/render_test.go`、`internal/bot/session_test.go`、`internal/bot/turn_dispatch_test.go`、`internal/bot/weixin/weixin_test.go`
- **需同步检查：** `internal/bot/gateway.go`、`internal/bot/gateway_test.go`、`internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`

#### 待修改项

- [ ] 先冻结 `*BotGateway.sessionOptionsForMessage` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L992、L994、L1006、L1013、L1019 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*BotGateway.sessionOptionsForMessage` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/bot/gateway.go|*BotGateway.sessionOptionsForMessage` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/gateway.go:988-1023` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*BotGateway.sessionOptionsForMessage` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/bot/gateway_test.go`。
- [ ] 运行相关包测试：`go test ./internal/bot`。
- [ ] 运行相关包 vet：`go vet ./internal/bot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/gateway.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/bot/gateway.go|*BotGateway.sessionOptionsForMessage` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-018 · 圈复杂度 · `isSafeSessionID`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-018 |
| Baseline key | `complexity\|internal/acp/service.go\|isSafeSessionID` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 24） |
| 文件 | `internal/acp/service.go` |
| 代码行 | 1456 |
| 函数/方法/包 | `isSafeSessionID` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`isSafeSessionID` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 1456-1473 行；可见控制流热点：L1457 `for _, r := range id {`；L1458 `if r >= 'a' && r <= 'z' {`；L1461 `if r >= 'A' && r <= 'Z' {`；L1464 `if r >= '0' && r <= '9' {`；L1467 `if r == '-' || r == '_' || r == '.' {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function isSafeSessionID（internal/acp/service.go:1456-1473）
- **可信生产调用方（1）：** `validateSessionID`（`internal/acp/service.go:1445`，调用点 L1450，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `service::sessionClose`（`internal/acp/service.go:793`，调用点 L798，confidence=0.90，exact-match，待源码复核）；`service::sessionDelete`（`internal/acp/service.go:862`，调用点 L867，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/acp`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`、`internal/acp/server_test.go`
- **需同步检查：** `internal/acp/service.go`、`internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`

#### 待修改项

- [ ] 先冻结 `isSafeSessionID` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1457、L1458、L1461、L1464、L1467 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `isSafeSessionID` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/acp/service.go|isSafeSessionID` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/acp/service.go:1456-1473` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `isSafeSessionID` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/acp/dispatch_partial_test.go`、`internal/acp/dispatch_test.go`、`internal/acp/e2e_test.go`、`internal/acp/live_test.go`、`internal/acp/protocol_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/acp`。
- [ ] 运行相关包 vet：`go vet ./internal/acp`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/acp/service.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/acp/service.go|isSafeSessionID` 下降或消失且无新增/恶化。
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
